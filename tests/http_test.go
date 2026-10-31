package test_sql

import (
	"bytes"
	"card_machine/internal/client"
	"card_machine/internal/handler"
	"card_machine/internal/model"
	usecases "card_machine/internal/useCases"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func testRouter(machine *usecases.NewCardMachineCase, pix *usecases.NewPaymenteCase, card *usecases.NewPaymentCardCase) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := handler.NewHandlerCase(machine, pix, card)
	r := gin.New()
	r.POST("/card_machine", h.InsertValueInCardMachine)
	r.POST("/Sale", h.PostSale)
	r.POST("/Pay/pix", h.PaymentPix)
	r.POST("/Pay/card", h.PaymentCard)
	return r
}

func serveJSON(t *testing.T, r http.Handler, path, payload string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, request)
	if !json.Valid(recorder.Body.Bytes()) {
		t.Fatalf("resposta não é JSON: HTTP %d", recorder.Code)
	}
	return recorder
}

func TestHttpInsertValue(t *testing.T) {
	const valid = `{"bank":"ITAU","person_name_storage":"Lojista Teste","storage_name":"Loja Teste","person_cpf":"12345678901","city":"Sao Paulo","state":"SP"}`
	for _, tt := range []struct {
		name, payload string
		duplicate     bool
		code, count   int
	}{
		{"cadastro completo", valid, false, 201, 1},
		{"bank obrigatório", strings.Replace(valid, `"bank":"ITAU",`, "", 1), false, 400, 0},
		{"CPF curto", strings.Replace(valid, "12345678901", "123", 1), false, 400, 0},
		{"JSON inválido", "{", false, 400, 0}, {"CPF repetido", valid, true, 409, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			connection, repo := newTestRepository(t)
			if tt.duplicate {
				insertTestMerchant(t, repo)
			}
			r := testRouter(usecases.NewCardMachineCaseDependency(repo, nil, nil, nil), nil, nil)
			result := serveJSON(t, r, "/card_machine", tt.payload)
			if result.Code != tt.code {
				t.Fatalf("HTTP %d; esperado %d", result.Code, tt.code)
			}
			var count int
			if err := connection.QueryRow("SELECT count(*) FROM bank_machine").Scan(&count); err != nil || count != tt.count {
				t.Fatal("tentativa inseriu quantidade inesperada de cadastros")
			}
			var body map[string]string
			json.Unmarshal(result.Body.Bytes(), &body)
			if tt.code == 201 && body["message"] != "Card machine initialized successfully" || tt.code != 201 && body["error"] == "" {
				t.Fatal("contrato de resposta do cadastro divergente")
			}
			t.Logf("Teste aprovado, valor: HTTP %d, cadastros=%d", result.Code, count)
		})
	}
}

func TestHttPostSale(t *testing.T) {
	for _, tt := range []struct {
		name, payload string
		pending       bool
	}{
		{name: "JSON inválido", payload: "{"}, {name: "tipo ausente", payload: `{"Amount":"25.33"}`},
		{name: "valor ausente", payload: `{"Type":"CARD","payment_type":"DEBIT"}`},
		{name: "valor numérico", payload: `{"Amount":25.33,"Type":"CARD","payment_type":"DEBIT"}`},
		{name: "valor inválido", payload: `{"Amount":"abc","Type":"CARD","payment_type":"DEBIT"}`},
		{name: "venda já pendente", payload: `{"Amount":"25.33","Type":"CARD","payment_type":"DEBIT"}`, pending: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			connection, repo := newTestRepository(t)
			expected := 0
			if tt.pending {
				insertTestSale(t, repo, model.PaymentCard, 25, 33)
				expected = 1
			}
			r := testRouter(usecases.NewCardMachineCaseDependency(repo, nil, nil, nil), nil, nil)
			result := serveJSON(t, r, "/Sale", tt.payload)
			if result.Code != 400 {
				t.Fatalf("HTTP %d; esperado 400", result.Code)
			}
			var count int
			if err := connection.QueryRow("SELECT count(*) FROM sales").Scan(&count); err != nil || count != expected {
				t.Fatal("requisição rejeitada criou venda indevida")
			}
			t.Logf("Teste aprovado, valor: HTTP 400, vendas=%d", count)
		})
	}
}

func TestPaymentInvalidJSON(t *testing.T) {
	valid := `{"PAN":"5201000000000006","dateExpirity":"31/12/31","Country":"076","Pin":{"PIN":"2468","CVM":"ONLINE_PIN"}}`
	for _, tt := range []struct{ name, path, payload string }{
		{"cartão JSON inválido", "/Pay/card", "{"}, {"cartão campos ausentes", "/Pay/card", "{}"},
		{"PAN ausente", "/Pay/card", strings.Replace(valid, `"PAN":"5201000000000006",`, "", 1)},
		{"validade no formato da PSP", "/Pay/card", strings.Replace(valid, "31/12/31", "311231", 1)},
		{"país curto", "/Pay/card", strings.Replace(valid, "076", "76", 1)},
		{"PIN numérico", "/Pay/card", strings.Replace(valid, `"PIN":"2468"`, `"PIN":2468`, 1)},
		{"Pix JSON inválido", "/Pay/pix", "{"}, {"Pix campos ausentes", "/Pay/pix", "{}"},
		{"Pix CPF curto", "/Pay/pix", `{"amount":"25.33","cpf":"123","name":"Cliente","bank":"ITAU"}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result := serveJSON(t, testRouter(nil, nil, nil), tt.path, tt.payload)
			if result.Code != 400 {
				t.Fatalf("HTTP %d; esperado 400", result.Code)
			}
			t.Log("Teste aprovado, valor: HTTP 400 antes de chamar o usecase")
		})
	}
}

func TestPaymentWithoutActiveSale(t *testing.T) {
	for _, tt := range []struct{ path, payload string }{
		{"/Pay/pix", `{"amount":"25.33","cpf":"12345678901","name":"Cliente","bank":"ITAU"}`},
		{"/Pay/card", `{"PAN":"5201000000000006","dateExpirity":"31/12/31","Country":"076","Pin":{"PIN":"2468","CVM":"ONLINE_PIN"}}`},
	} {
		t.Run(tt.path, func(t *testing.T) {
			r := testRouter(nil, usecases.NewPaymentCase(nil, nil), usecases.NewPaymentCardDependencyCase(nil, nil, nil))
			result := serveJSON(t, r, tt.path, tt.payload)
			if result.Code != 409 {
				t.Fatalf("HTTP %d; esperado 409 para ausência de listener", result.Code)
			}
			t.Log("Teste aprovado, valor: HTTP 409 sem venda ativa")
		})
	}
}

// Fluxo da rota até a PSP simulada: DEBIT/CREDIT, decisão, log, histórico e eventos.
func TestPaymentSaleCard(t *testing.T) {
	for _, kind := range []model.CardPaymentType{model.Debit, model.Credit} {
		for _, approved := range []bool{true, false} {
			name := string(kind) + "/reprovado"
			if approved {
				name = string(kind) + "/aprovado"
			}
			t.Run(name, func(t *testing.T) {
				testEnvironment(t)
				connection, repo := newTestRepository(t)
				insertTestMerchant(t, repo)
				cluster, producer, events := testKafka(t)
				status, reason, code := model.StatusReproved, "Saldo insuficiente.", 400
				if kind == model.Credit {
					reason = "Limite de crédito insuficiente."
				}
				if approved {
					status, reason, code = model.StatusApproved, "", 200
				}
				expected := model.AuthorizationResponse{Status: status, PaymentType: kind, CardNetwork: "MASTERCARD", IssuerBank: "ITAU", Amount: "25.33", TimeNow: testTime, Log: reason}
				sent := make(chan model.CardPayment, 1)
				card := usecases.NewPaymentCardDependencyCase(repo, events, testPSP(t, expected, func(p model.CardPayment) { sent <- p }))
				pix := usecases.NewPaymentCase(repo, events)
				machine := usecases.NewCardMachineCaseDependency(repo, pix, card, events)
				r := testRouter(machine, pix, card)
				sale := serveJSON(t, r, "/Sale", fmt.Sprintf(`{"Amount":"25.33","Type":"CARD","payment_type":%q}`, kind))
				if sale.Code != 201 || !bytes.Contains(sale.Body.Bytes(), []byte(`"sucess":"PENDING"`)) {
					t.Fatal("criação deve retornar PENDING, nunca aprovação")
				}
				assertSaleState(t, connection, 1, model.StatusPending, false)
				payload, _ := json.Marshal(testTags())
				result := serveJSON(t, r, "/Pay/card", string(payload))
				var body map[string]string
				if json.Unmarshal(result.Body.Bytes(), &body) != nil || result.Code != code || len(body) != 2 || body["status"] != string(status) || body["log"] != reason {
					t.Fatalf("resposta divergente: HTTP %d %s", result.Code, result.Body.String())
				}
				for _, secret := range []string{testPAN, testPIN, testKey} {
					if strings.Contains(result.Body.String(), secret) {
						t.Fatal("resposta expôs dados sensíveis")
					}
				}
				select {
				case payment := <-sent:
					verifyCardPayload(t, payment, kind)
				case <-time.After(time.Second):
					t.Fatal("a PSP não recebeu autorização")
				}
				awaitSaleStatus(t, connection, 1, status)
				assertSaleState(t, connection, 1, status, approved)
				receipt, err := os.ReadFile("comprovante_card.txt")
				if approved {
					if err != nil || !bytes.Contains(receipt, []byte("25.33")) || !bytes.Contains(receipt, []byte(kind)) {
						t.Fatal("aprovação não gerou comprovante com valor e tipo")
					}
				} else if !os.IsNotExist(err) {
					t.Fatal("reprovação não deve gerar comprovante")
				}
				received := kafkaEvents(t, cluster, producer, 2)
				if received[0].SaleId != 1 || received[0].Value != "25.33" || received[0].Status != model.StatusPending || received[1].SaleId != 1 || received[1].Status != status {
					t.Fatal("eventos não refletem entrada e decisão da venda")
				}
				t.Logf("Teste aprovado, valor: %s, tipo=%s, log=%q", status, kind, reason)
			})
		}
	}
}

func TestPixSaleHTTP(t *testing.T) {
	testEnvironment(t)
	connection, repo := newTestRepository(t)
	insertTestMerchant(t, repo)
	cluster, producer, events := testKafka(t)
	pix := usecases.NewPaymentCase(repo, events)
	r := testRouter(usecases.NewCardMachineCaseDependency(repo, pix, nil, events), pix, nil)
	created := serveJSON(t, r, "/Sale", `{"Amount":"25.33","Type":"PIX"}`)
	if created.Code != 201 {
		t.Fatal("venda Pix não foi criada")
	}
	qr, err := os.ReadFile("qr.png")
	if err != nil || !bytes.HasPrefix(qr, []byte{0x89, 'P', 'N', 'G'}) {
		t.Fatal("QR Code PNG não foi gerado")
	}
	result := serveJSON(t, r, "/Pay/pix", `{"amount":"25.33","cpf":"12345678901","name":"Cliente Teste","bank":"ITAU"}`)
	if result.Code != 200 || result.Body.String() != `{"status":"APPROVED"}` {
		t.Fatalf("contrato de aprovação Pix divergente: %s", result.Body.String())
	}
	assertSaleState(t, connection, 1, model.StatusApproved, true)
	got := kafkaEvents(t, cluster, producer, 2)
	if got[0].Status != model.StatusPending || got[1].Status != model.StatusApproved {
		t.Fatal("eventos do Pix não refletem o ciclo completo")
	}
	t.Log("Teste aprovado, valor: APPROVED com QR Code, comprovante e eventos Pix")
}

type trackedBody struct {
	io.Reader
	closed bool
}

func (b *trackedBody) Close() error { b.closed = true; return nil }

func TestPSPClient(t *testing.T) {
	approved := model.AuthorizationResponse{Status: model.StatusApproved, PaymentType: model.Debit, CardNetwork: "MASTERCARD", IssuerBank: "ITAU", Amount: "25.33", TimeNow: testTime}
	declined := approved
	declined.Status = model.StatusReproved
	declined.Log = "PIN incorreto."
	good, _ := json.Marshal(approved)
	bad, _ := json.Marshal(declined)
	transportErr := errors.New("transporte indisponível")
	for _, tt := range []struct {
		name, body   string
		code         int
		transportErr error
		want         model.AuthorizationResponse
		wantErr      bool
	}{
		{name: "aprovado", code: 200, body: string(good), want: approved},
		{name: "reprovado preserva resposta", code: 200, body: string(bad), want: declined, wantErr: true},
		{name: "JSON inválido", code: 200, body: "{", wantErr: true}, {name: "corpo vazio", code: 200, wantErr: true},
		{name: "erro de transporte", transportErr: transportErr, wantErr: true},
		{name: "HTTP 500 não pode aprovar", code: 500, body: string(good), wantErr: true},
		{name: "HTTP 401 não pode aprovar", code: 401, body: string(good), wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body := &trackedBody{Reader: strings.NewReader(tt.body)}
			psp := &client.PSPClient{BaseURL: "http://psp.test", HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if tt.transportErr != nil {
					return nil, tt.transportErr
				}
				return &http.Response{StatusCode: tt.code, Header: make(http.Header), Body: body}, nil
			})}}
			got, err := psp.PSP(model.CardPayment{})
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Fatalf("resposta=%+v, erro=%v; esperado erro=%t", got, err, tt.wantErr)
			}
			if tt.transportErr == nil && !body.closed {
				t.Fatal("client não fechou o corpo HTTP")
			}
			if tt.transportErr != nil && !errors.Is(err, transportErr) {
				t.Fatal("erro de transporte original não foi propagado")
			}
			t.Logf("Teste aprovado, valor: status=%s, erro=%t", got.Status, err != nil)
		})
	}
	psp := &client.PSPClient{BaseURL: "://inválido", HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Error("URL inválida não deve chegar ao transporte")
		return nil, transportErr
	})}}
	if _, err := psp.PSP(model.CardPayment{}); err == nil {
		t.Fatal("URL inválida deve retornar erro")
	}
}

// Mantém o teste de regressão solicitado anteriormente, agora coberto no fluxo completo.
func TestPaymentCardReprovedLog(t *testing.T) {
	testEnvironment(t)
	_, repo := newTestRepository(t)
	expected := model.AuthorizationResponse{Status: model.StatusReproved, PaymentType: model.Debit, Amount: "25.33", TimeNow: testTime, Log: "Saldo insuficiente."}
	usecase := usecases.NewPaymentCardDependencyCase(repo, nil, testPSP(t, expected, nil))
	result, err := usecase.PaymentSaleCard(testTags(), model.CardSale{SaleId: 1, Amount: "25.33", TimeNow: testTime, Status: model.StatusPending, PaymentType: model.Debit}, 1)
	if err == nil || result != expected {
		t.Fatal("usecase descartou a resposta ou o log da PSP")
	}
	t.Logf("Teste aprovado, valor: %s, log=%s", result.Status, result.Log)
}
