package test_sql

import (
	"bytes"
	"card_machine/internal/client"
	"card_machine/internal/kafka_config"
	"card_machine/internal/model"
	usecases "card_machine/internal/useCases"
	"crypto/aes"
	"crypto/cipher"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"testing/synctest"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/google/uuid"
	"github.com/hugoferreiraTI/AES_CMAC_Go/cmac"
)

const testKey = "00112233445566778899aabbccddeeff"
const testTime = "2030-10-31T12:00:00-03:00"
const testPAN = "5201000000000006"
const testPIN = "2468"

// Arquivos e chaves existem só no ambiente temporário de cada teste.
func testEnvironment(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
	t.Setenv("AES_CMAC_KEY", testKey)
	t.Setenv("AES_GCM", testKey)
	if err := os.WriteFile(".env", []byte("# Ambiente temporário do teste\n"), 0600); err != nil {
		t.Fatal(err)
	}
}

func testTags() model.EmvTags {
	return model.EmvTags{PAN: testPAN, DateExpiry: "31/12/31", TransactionCountry: "076", Pin: model.PIN{PIN: testPIN, CVM: model.CVMOnlinePIN}}
}

// RoundTripper em memória: nenhum teste envia pagamentos à PSP real.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func testPSP(t *testing.T, result model.AuthorizationResponse, inspect func(model.CardPayment)) *client.PSPClient {
	t.Helper()
	return &client.PSPClient{BaseURL: "http://psp.test", HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost || r.URL.Path != "/authorize/card" || r.Header.Get("Content-Type") != "application/json" {
			t.Error("contrato HTTP da PSP incorreto")
		}
		var payment model.CardPayment
		if err := json.NewDecoder(r.Body).Decode(&payment); err != nil {
			t.Error("requisição não contém JSON válido")
		}
		defer r.Body.Close()
		if inspect != nil {
			inspect(payment)
		}
		response := result
		response.TimeNow = payment.TimeNow
		body, err := json.Marshal(response)
		if err != nil {
			return nil, err
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(body))}, nil
	})}}
}

func testKafka(t *testing.T) (*kafka.MockCluster, *kafka.Producer, *kafka_config.NewProducerCase) {
	t.Helper()
	cluster, err := kafka.NewMockCluster(1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cluster.Close)
	producer, err := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": cluster.BootstrapServers(), "acks": "all", "message.timeout.ms": 2000})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { producer.Flush(3000); producer.Close() })
	return cluster, producer, kafka_config.NewProducerCaseDependency(producer)
}

func kafkaEvents(t *testing.T, cluster *kafka.MockCluster, producer *kafka.Producer, count int) []model.PaymentKafka {
	t.Helper()
	if pending := producer.Flush(3000); pending != 0 {
		t.Fatalf("%d eventos ficaram sem entrega", pending)
	}
	consumer, err := kafka.NewConsumer(&kafka.ConfigMap{"bootstrap.servers": cluster.BootstrapServers(), "group.id": "unit-tests"})
	if err != nil {
		t.Fatal(err)
	}
	defer consumer.Close()
	topic := "sale"
	metadata, err := consumer.GetMetadata(&topic, false, 3000)
	if err != nil {
		t.Fatal(err)
	}
	var partitions []kafka.TopicPartition
	for _, p := range metadata.Topics[topic].Partitions {
		partitions = append(partitions, kafka.TopicPartition{Topic: &topic, Partition: p.ID, Offset: kafka.OffsetBeginning})
	}
	if err := consumer.Assign(partitions); err != nil {
		t.Fatal(err)
	}
	var result []model.PaymentKafka
	deadline := time.Now().Add(4 * time.Second)
	for len(result) < count && time.Now().Before(deadline) {
		message, err := consumer.ReadMessage(100 * time.Millisecond)
		if err != nil {
			continue
		}
		var event model.PaymentKafka
		if err := json.Unmarshal(message.Value, &event); err != nil {
			t.Fatal(err)
		}
		if string(message.Key) != fmt.Sprint(event.SaleId) {
			t.Fatal("evento Kafka usa chave diferente do ID da venda")
		}
		result = append(result, event)
	}
	if len(result) != count {
		t.Fatalf("recebidos %d eventos; esperado %d", len(result), count)
	}
	return result
}

func decryptTestPIN(t *testing.T, payload string) string {
	t.Helper()
	key, _ := hex.DecodeString(testKey)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	data, err := hex.DecodeString(payload)
	if err != nil || len(data) < gcm.NonceSize()+gcm.Overhead() {
		t.Fatal("payload GCM malformado")
	}
	pin, err := gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], nil)
	if err != nil {
		t.Fatal("PIN não autenticado pelo GCM")
	}
	return string(pin)
}

func verifyCardPayload(t *testing.T, p model.CardPayment, kind model.CardPaymentType) {
	t.Helper()
	if p.SaleId <= 0 || p.Amount != "25.33" || p.PaymentType != kind || p.Status != model.StatusPending {
		t.Error("venda ou tipo alterado ao montar CardPayment")
	}
	if _, err := time.Parse(time.RFC3339, p.TimeNow); err != nil {
		t.Error("horário deve usar RFC3339")
	}
	if !bytes.Equal(p.Emv.PAN.Value, []byte{0x52, 0x01, 0, 0, 0, 0, 0, 6}) {
		t.Error("PAN não foi codificado corretamente")
	}
	if decryptTestPIN(t, p.Emv.Pin.PIN) != testPIN {
		t.Error("GCM não recuperou o PIN original")
	}
	var number [4]byte
	copy(number[:], p.Emv.NumberUnpredictable.Value)
	message := fmt.Sprintf("%s|%s|%s|%s", p.Status, p.Amount, p.TimeNow, number)
	valid, err := cmac.Verify(testKey, message, hex.EncodeToString(p.Emv.Cryptogram.Value))
	if err != nil || !valid {
		t.Error("CMAC não corresponde à venda enviada")
	}
}

func TestMerchantUsecase(t *testing.T) {
	connection, repo := newTestRepository(t)
	usecase := usecases.NewCardMachineCaseDependency(repo, nil, nil, nil)
	merchant := testMerchant()
	merchant.UUID = ""
	merchant.SerialNumber = ""
	if _, err := usecase.InsertValueInCardMachine(&merchant); err != nil {
		t.Fatal(err)
	}
	if _, err := uuid.Parse(merchant.UUID); err != nil {
		t.Fatal("UUID da maquininha não foi gerado")
	}
	if _, err := uuid.Parse(merchant.SerialNumber); err != nil {
		t.Fatal("número de série não foi gerado")
	}
	if _, err := usecase.InsertValueInCardMachine(&merchant); err == nil {
		t.Fatal("CPF duplicado deveria ser rejeitado")
	}
	var count int
	if err := connection.QueryRow("SELECT count(*) FROM bank_machine").Scan(&count); err != nil || count != 1 {
		t.Fatal("tentativa duplicada inseriu novo cadastro")
	}
	t.Log("Teste aprovado, valor: cadastro único com UUID e série gerados")
}

func TestSaleAmount(t *testing.T) {
	testEnvironment(t)
	_, _, events := testKafka(t)
	for _, tt := range []struct {
		amount      string
		real, cents int64
		invalid     bool
	}{
		{"25.33", 25, 33, false}, {"25", 25, 0, false}, {"25.3", 25, 30, false}, {"0.01", 0, 1, false},
		{"", 0, 0, true}, {"25,33", 0, 0, true}, {"abc", 0, 0, true}, {"25.333", 0, 0, true}, {"1.2.3", 0, 0, true}, {"9223372036854775808", 0, 0, true},
	} {
		t.Run(tt.amount, func(t *testing.T) {
			connection, repo := newTestRepository(t)
			card := usecases.NewPaymentCardDependencyCase(repo, events, testPSP(t, model.AuthorizationResponse{Status: model.StatusReproved}, nil))
			usecase := usecases.NewCardMachineCaseDependency(repo, nil, card, events)
			result, err := usecase.PostSale(model.Sale{Amount: tt.amount, PayType: model.PaymentCard, PaymentType: model.Debit})
			var count int
			if scanErr := connection.QueryRow("SELECT count(*) FROM sales").Scan(&count); scanErr != nil {
				t.Fatal(scanErr)
			}
			if tt.invalid {
				if err == nil || count != 0 {
					t.Fatal("valor inválido deve retornar erro sem criar venda")
				}
				t.Log("Teste aprovado, valor: entrada monetária inválida rejeitada")
				return
			}
			if err != nil || result != "PENDING" || count != 1 {
				t.Fatalf("criação inesperada: resultado=%s erro=%v", result, err)
			}
			real, cents, err := repo.RecoverValues(1)
			if err != nil || real != tt.real || cents != tt.cents {
				t.Fatal("conversão monetária alterou reais ou centavos")
			}
			// Finaliza o listener iniciado pelo usecase, sem deixar goroutine esperando 30s.
			response := make(chan model.AuthorizationResponse, 1)
			select {
			case card.PaymentChan <- usecases.PaymentCard{Tag: testTags(), Response: response}:
			case <-time.After(2 * time.Second):
				t.Fatal("listener não recebeu tentativa de finalização")
			}
			select {
			case <-response:
			case <-time.After(2 * time.Second):
				t.Fatal("listener não respondeu")
			}
			awaitSaleStatus(t, connection, 1, model.StatusReproved)
			t.Logf("Teste aprovado, valor: %d.%02d", real, cents)
		})
	}
}

// Vetor conhecido calculado de forma independente para o protocolo de quatro campos.
func TestCrypto(t *testing.T) {
	testEnvironment(t)
	usecase := usecases.NewPaymentCardDependencyCase(nil, nil, nil)
	data := model.Cryptogram{StatusCard: model.StatusPending, AmountCrypt: "25.33", TimeNowCrypt: testTime, NumberUnpredictableCrypt: [4]byte{0, 0x7c, 0x80, 0xff}}
	got, err := usecase.Crypto(data)
	if err != nil || got != "0a9a415dd6449f93b29014e3428fcacf" {
		t.Fatal("CMAC não corresponde ao vetor conhecido de 128 bits")
	}
	for _, tt := range []struct {
		name   string
		change func(*model.Cryptogram)
	}{
		{"valor", func(p *model.Cryptogram) { p.AmountCrypt = "25.34" }}, {"status", func(p *model.Cryptogram) { p.StatusCard = model.StatusApproved }},
		{"horário", func(p *model.Cryptogram) { p.TimeNowCrypt = "2030-10-31T12:00:01-03:00" }}, {"número", func(p *model.Cryptogram) { p.NumberUnpredictableCrypt[0] = 1 }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			changed := data
			tt.change(&changed)
			value, err := usecase.Crypto(changed)
			if err != nil || value == got {
				t.Fatal("alteração de campo assinado deve alterar CMAC")
			}
			t.Log("Teste aprovado, valor: alteração detectável pelo CMAC")
		})
	}
	t.Setenv("AES_CMAC_KEY", "chave-inválida")
	if _, err := usecase.Crypto(data); err == nil {
		t.Fatal("chave CMAC inválida deve retornar erro")
	}
	t.Log("Teste aprovado, valor: AES-CMAC de 128 bits validado por vetor conhecido")
}

func TestTlv(t *testing.T) {
	testEnvironment(t)
	usecase := usecases.NewPaymentCardDependencyCase(nil, nil, nil)
	const mac = "0a9a415dd6449f93b29014e3428fcacf"
	number := [4]byte{0, 0x7c, 0x80, 0xff}
	tlv, err := usecase.Tlv(testTags(), mac, number)
	if err != nil {
		t.Fatal(err)
	}
	expectedMAC, _ := hex.DecodeString(mac)
	for _, tt := range []struct {
		name       string
		got        model.TLV
		tag, value []byte
	}{
		{"PAN", tlv.PAN, []byte{0x5a}, []byte{0x52, 0x01, 0, 0, 0, 0, 0, 6}},
		{"validade", tlv.DateExpiry, []byte{0x5f, 0x24}, []byte{0x31, 0x12, 0x31}},
		{"país", tlv.TransactionCountry, []byte{0x9f, 0x1a}, []byte{0, 0x76}},
		{"criptograma", tlv.Cryptogram, []byte{0x9f, 0x26}, expectedMAC},
		{"número imprevisível", tlv.NumberUnpredictable, []byte{0x9f, 0x37}, number[:]},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if !bytes.Equal(tt.got.Tag, tt.tag) || !bytes.Equal(tt.got.Value, tt.value) || tt.got.Length != len(tt.value) {
				t.Fatal("tag, comprimento ou conteúdo TLV divergente")
			}
			t.Logf("Teste aprovado, valor: %s com %d bytes", tt.name, tt.got.Length)
		})
	}
	if decryptTestPIN(t, tlv.Pin.PIN) != testPIN || tlv.Pin.CVM != model.CVMOnlinePIN {
		t.Fatal("PIN ou CVM alterado")
	}
	again, err := usecase.Tlv(testTags(), mac, number)
	if err != nil || again.Pin.PIN == tlv.Pin.PIN {
		t.Fatal("novas criptografias devem usar nonces diferentes")
	}
	raw, err := hex.DecodeString(tlv.Pin.PIN)
	if err != nil {
		t.Fatal(err)
	}
	raw[len(raw)-1] ^= 1
	key, _ := hex.DecodeString(testKey)
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	if _, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil); err == nil {
		t.Fatal("GCM deve rejeitar payload adulterado")
	}
	encoded, err := json.Marshal(tlv.PAN)
	if err != nil {
		t.Fatal(err)
	}
	var jsonPAN struct {
		Tag    string
		Length int
		Value  string
	}
	if json.Unmarshal(encoded, &jsonPAN) != nil || jsonPAN.Tag != "Wg==" || jsonPAN.Value != "UgEAAAAAAAY=" || jsonPAN.Length != 8 {
		t.Fatal("JSON precisa codificar bytes em Base64")
	}
	for _, tt := range []struct {
		pan, hex string
		length   int
	}{
		{"520100000000", "520100000000", 6}, {"5201000000000000006", "5201000000000000006f", 10},
	} {
		tags := testTags()
		tags.PAN = tt.pan
		result, err := usecase.Tlv(tags, mac, number)
		if err != nil || hex.EncodeToString(result.PAN.Value) != tt.hex || result.PAN.Length != tt.length {
			t.Fatal("PAN nos limites de tamanho ou padding F incorretos")
		}
	}
	t.Log("Teste aprovado, valor: TLV correto, GCM autenticado e JSON em Base64")
}

func TestTlvInvalidInput(t *testing.T) {
	testEnvironment(t)
	usecase := usecases.NewPaymentCardDependencyCase(nil, nil, nil)
	for _, tt := range []struct {
		name   string
		change func(*model.EmvTags)
		mac    string
	}{
		{name: "PAN curto", change: func(p *model.EmvTags) { p.PAN = "123" }},
		{name: "PAN longo", change: func(p *model.EmvTags) { p.PAN = "52010000000000000000" }},
		{name: "PAN alfanumérico", change: func(p *model.EmvTags) { p.PAN = "520100000000000A" }},
		{name: "PAN com espaço", change: func(p *model.EmvTags) { p.PAN = "520100000000 006" }},
		{name: "PAN Unicode", change: func(p *model.EmvTags) { p.PAN = "520100000000000６" }},
		{name: "país curto", change: func(p *model.EmvTags) { p.TransactionCountry = "76" }},
		{name: "país inválido", change: func(p *model.EmvTags) { p.TransactionCountry = "0A6" }},
		{name: "validade sem barras", change: func(p *model.EmvTags) { p.DateExpiry = "311231" }},
		{name: "dia curto", change: func(p *model.EmvTags) { p.DateExpiry = "1/12/31" }},
		{name: "dia zero", change: func(p *model.EmvTags) { p.DateExpiry = "00/12/31" }},
		{name: "dia acima de 31", change: func(p *model.EmvTags) { p.DateExpiry = "32/12/31" }},
		{name: "mês zero", change: func(p *model.EmvTags) { p.DateExpiry = "31/00/31" }},
		{name: "mês acima de 12", change: func(p *model.EmvTags) { p.DateExpiry = "31/13/31" }},
		{name: "ano longo", change: func(p *model.EmvTags) { p.DateExpiry = "31/12/2031" }},
		{name: "ano fora do MVP", change: func(p *model.EmvTags) { p.DateExpiry = "31/12/32" }},
		{name: "CMAC não hexadecimal", mac: "xyz"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tags := testTags()
			if tt.change != nil {
				tt.change(&tags)
			}
			mac := tt.mac
			if mac == "" {
				mac = "0a9a415dd6449f93b29014e3428fcacf"
			}
			if _, err := usecase.Tlv(tags, mac, [4]byte{}); err == nil {
				t.Fatal("entrada inválida foi aceita")
			}
			t.Log("Teste aprovado, valor: entrada inválida rejeitada")
		})
	}
	t.Setenv("AES_GCM", "xyz")
	if _, err := usecase.Tlv(testTags(), "0a9a415dd6449f93b29014e3428fcacf", [4]byte{}); err == nil {
		t.Fatal("chave GCM inválida deve retornar erro")
	}
	if err := os.Remove(".env"); err != nil {
		t.Fatal(err)
	}
	if _, err := usecase.Crypto(model.Cryptogram{}); err == nil {
		t.Fatal("ausência de configuração deve retornar erro")
	}
	t.Log("Teste aprovado, valor: entradas e configurações inválidas rejeitadas")
}

func TestPaymentPix(t *testing.T) {
	testEnvironment(t)
	connection, repo := newTestRepository(t)
	insertTestMerchant(t, repo)
	cluster, producer, events := testKafka(t)
	payment := usecases.NewPaymentCase(repo, events)
	id := insertTestSale(t, repo, model.PaymentPix, 25, 33)
	done := make(chan error, 1)
	go func() { _, err := payment.WaitPaytament(payment.PaymentChan, id); done <- err }()
	for _, tt := range []struct {
		amount string
		want   model.SaleStatus
	}{{"25.32", model.StatusReproved}, {"25.33", model.StatusApproved}} {
		reply := make(chan model.SaleStatus, 1)
		select {
		case payment.PaymentChan <- usecases.PaymentAttempt{Pix: model.Pix{AmountPix: tt.amount, Cpf: testMerchant().PersonCpf, Name: "Cliente Teste", Bank: "ITAU"}, Response: reply}:
		case <-time.After(2 * time.Second):
			t.Fatal("listener Pix não recebeu tentativa")
		}
		select {
		case status := <-reply:
			if status != tt.want {
				t.Fatalf("status=%s; esperado=%s", status, tt.want)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("listener não respondeu")
		}
		if tt.want == model.StatusReproved {
			assertSaleState(t, connection, id, model.StatusPending, false)
			if _, err := os.Stat("comprovante.txt"); !os.IsNotExist(err) {
				t.Fatal("Pix recusado não deveria gerar comprovante")
			}
		}
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("listener não encerrou após aprovação")
	}
	assertSaleState(t, connection, id, model.StatusApproved, true)
	receipt, err := os.ReadFile("comprovante.txt")
	if err != nil || !bytes.Contains(receipt, []byte("25.33")) {
		t.Fatal("comprovante Pix não foi gerado com o valor correto")
	}
	got := kafkaEvents(t, cluster, producer, 2)
	if got[0].Status != model.StatusReproved || got[1].Status != model.StatusApproved {
		t.Fatal("eventos Pix não refletem rejeição e aprovação")
	}
	t.Log("Teste aprovado, valor: REPROVED manteve PENDING, nova tentativa APPROVED")
}

// Usado pelos testes HTTP para aguardar apenas a persistência assíncrona de recusa.
func awaitSaleStatus(t *testing.T, connection *sql.DB, id int, want model.SaleStatus) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		var status model.SaleStatus
		if err := connection.QueryRow("SELECT status FROM sales WHERE id=?", id).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if status == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("histórico não atingiu o estado %s", want)
}

// O relógio virtual testa os 30s de espera sem tornar a suíte lenta.
func TestPaymentTimeout(t *testing.T) {
	for _, kind := range []model.PaymentType{model.PaymentPix, model.PaymentCard} {
		t.Run(string(kind), func(t *testing.T) {
			connection, repo := newTestRepository(t)
			id := insertTestSale(t, repo, kind, 25, 33)
			cluster, err := kafka.NewMockCluster(1)
			if err != nil {
				t.Fatal(err)
			}
			defer cluster.Close()
			producer, err := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": cluster.BootstrapServers(), "message.timeout.ms": 1000})
			if err != nil {
				t.Fatal(err)
			}
			events := kafka_config.NewProducerCaseDependency(producer)
			synctest.Test(t, func(t *testing.T) {
				// O leitor de confirmações iniciado na bolha também precisa ser encerrado.
				defer producer.Close()
				start := time.Now()
				var status model.SaleStatus
				var err error
				if kind == model.PaymentPix {
					status, err = usecases.NewPaymentCase(repo, events).WaitPaytament(make(chan usecases.PaymentAttempt), id)
				} else {
					status, err = usecases.NewPaymentCardDependencyCase(repo, events, nil).WaitPaymentCard(make(chan usecases.PaymentCard), model.CardSale{}, id)
				}
				if err != nil || status != model.StatusExpired || time.Since(start) != 30*time.Second {
					t.Fatalf("timeout inesperado: status=%s erro=%v tempo=%s", status, err, time.Since(start))
				}
			})
			assertSaleState(t, connection, id, model.StatusExpired, false)
			t.Log("Teste aprovado, valor: EXPIRED após 30 segundos virtuais, sem horário de aprovação")
		})
	}
}

func TestPaymentCardErrors(t *testing.T) {
	for _, tt := range []struct {
		name     string
		change   func(*model.EmvTags)
		env, key string
		network  bool
	}{
		{name: "chave CMAC inválida", env: "AES_CMAC_KEY", key: "xyz"},
		{name: "chave GCM inválida", env: "AES_GCM", key: "xyz"},
		{name: "tamanho AES inválido", env: "AES_GCM", key: "00"},
		{name: "PAN inválido", change: func(p *model.EmvTags) { p.PAN = "123" }},
		{name: "validade inválida", change: func(p *model.EmvTags) { p.DateExpiry = "311231" }},
		{name: "PSP indisponível", network: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			testEnvironment(t)
			if tt.env != "" {
				t.Setenv(tt.env, tt.key)
			}
			connection, repo := newTestRepository(t)
			id := insertTestSale(t, repo, model.PaymentCard, 25, 33)
			calls := 0
			psp := &client.PSPClient{BaseURL: "http://psp.test", HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { calls++; return nil, errors.New("PSP indisponível") })}}
			usecase := usecases.NewPaymentCardDependencyCase(repo, nil, psp)
			tags := testTags()
			if tt.change != nil {
				tt.change(&tags)
			}
			result, err := usecase.PaymentSaleCard(tags, model.CardSale{SaleId: id, Amount: "25.33", TimeNow: testTime, Status: model.StatusPending, PaymentType: model.Debit}, id)
			if err == nil || result.Status != model.StatusReproved {
				t.Fatal("falha de preparação ou transporte não deve aprovar cartão")
			}
			wantCalls := 0
			if tt.network {
				wantCalls = 1
			}
			if calls != wantCalls {
				t.Fatal("requisição enviada à PSP antes de concluir a preparação")
			}
			// A persistência de REPROVED é responsabilidade do WaitPaymentCard.
			assertSaleState(t, connection, id, model.StatusPending, false)
			if _, err := os.Stat("comprovante_card.txt"); !os.IsNotExist(err) {
				t.Fatal("falha não pode gerar comprovante de aprovação")
			}
			t.Logf("Teste aprovado, valor: %s, chamadas PSP=%d", result.Status, calls)
		})
	}
}
