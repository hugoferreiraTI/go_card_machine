package test_sql

import (
	"bytes"
	"card_machine/internal/handler"
	"card_machine/internal/kafka_config"
	"card_machine/internal/model"
	"card_machine/internal/repository"
	usecases "card_machine/internal/useCases"
	"encoding/hex"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

func TestPutStatus(t *testing.T) {
	value := model.Pix{
		AmountPix: "100.10",
		Cpf:       "12345678912",
		Name:      "diego",
		Bank:      "nubank",
	}
	var id int
	id = 1

	// Call the DbConnection function to test the database connection
	db, err, _ := repository.DbConnection()
	if err != nil {
		fmt.Print(err)
		t.Fatal(err)
		return
	}
	produce, err := kafka.NewProducer(&kafka.ConfigMap{
		// User-specific properties that you must set
		"bootstrap.servers": "localhost:9092", //run in docker local
		// Fixed properties
		"acks": "all"})

	if err != nil {
		fmt.Printf("Failed to create producer: %s", err)
		os.Exit(1)
	}

	//Initialize the repository with dependency
	repo := repository.NewRepository(db)
	kafkaCase := kafka_config.NewProducerCaseDependency(produce)
	paymentCase := usecases.NewPaymentCase(repo, kafkaCase)
	authCase := usecases.NewCardMachineCaseDependency(repo, paymentCase, kafkaCase)
	_ = handler.NewHandlerCase(authCase, paymentCase)

	status, err := paymentCase.Payment(value, id)

	if err != nil {
		fmt.Print(status)
		fmt.Print(err)
		t.Fatal(err)
		return
	}

	fmt.Print(err)
	fmt.Print(status)

}

func TestPaymentPix(t *testing.T) {
	amount := make(chan usecases.PaymentAttempt)
	responseChan := make(chan model.SaleStatus)

	go func() {
		amount <- usecases.PaymentAttempt{
			Pix: model.Pix{
				AmountPix: "200.00",
				Cpf:       "12345678912",
				Name:      "Hugo",
				Bank:      "nubank",
			},
			Response: responseChan,
		}
	}()
	id := 4

	// Call the DbConnection function to test the database connection
	db, err, _ := repository.DbConnection()
	if err != nil {
		fmt.Print(err)
		t.Fatal(err)
		return
	}

	produce, err := kafka.NewProducer(&kafka.ConfigMap{
		// User-specific properties that you must set
		"bootstrap.servers": "localhost:9092", //run in docker local
		// Fixed properties
		"acks": "all"})

	if err != nil {
		fmt.Printf("Failed to create producer: %s", err)
		os.Exit(1)
	}

	//Initialize the repository with dependency
	repo := repository.NewRepository(db)
	kafkaCase := kafka_config.NewProducerCaseDependency(produce)
	paymentCase := usecases.NewPaymentCase(repo, kafkaCase)
	authCase := usecases.NewCardMachineCaseDependency(repo, paymentCase, kafkaCase)
	_ = handler.NewHandlerCase(authCase, paymentCase)

	status, err := paymentCase.WaitPaytament(amount, id)

	if err != nil {
		fmt.Print("paymente repproved")
		return
	}

	if status == model.StatusApproved {
		fmt.Print("payment approved")
		return
	}

	fmt.Print(status)

}

func TestProcessSaleCard(t *testing.T) {
	// Call the DbConnection function to test the database connection
	db, err, _ := repository.DbConnection()
	if err != nil {
		fmt.Print(err)
		t.Fatal(err)
		return
	}
	produce, err := kafka.NewProducer(&kafka.ConfigMap{
		// User-specific properties that you must set
		"bootstrap.servers": "localhost:9092", //run in docker local
		// Fixed properties
		"acks": "all"})

	if err != nil {
		fmt.Printf("Failed to create producer: %s", err)
		os.Exit(1)
	}

	//Initialize the repository with dependency
	repo := repository.NewRepository(db)
	kafkaCase := kafka_config.NewProducerCaseDependency(produce)
	paymentCase := usecases.NewPaymentCase(repo, kafkaCase)
	paymentCardCase := usecases.NewPaymentCardDependencyCase(repo, kafkaCase)
	authCase := usecases.NewCardMachineCaseDependency(repo, paymentCase, kafkaCase)
	_ = handler.NewHandlerCase(authCase, paymentCase)

	var real int64
	var cents int64

	real = 20
	cents = 10
	id := 1
	status, err := paymentCardCase.ProcessSaleCard(real, cents, id)

	if err != nil {
		fmt.Printf("Failed to a crypto: %s", err)
		os.Exit(1)
	}

	fmt.Print(status)

}

func TestCrypto(t *testing.T) {
	const testKey = "00112233445566778899aabbccddeeff"

	oldEnv, err := os.ReadFile(".env")
	envExisted := err == nil

	err = os.WriteFile(
		".env",
		[]byte("AES_CMAC_KEY="+testKey+"\n"),
		0600,
	)
	if err != nil {
		t.Fatalf("error creating test .env: %v", err)
	}

	defer func() {
		if envExisted {
			_ = os.WriteFile(".env", oldEnv, 0600)
		} else {
			_ = os.Remove(".env")
		}
	}()

	paymentCardCase := usecases.NewPaymentCardDependencyCase(nil, nil)

	unpredictableNumber, err := paymentCardCase.GenerateUnpredictableNumber()
	if err != nil {
		t.Fatalf("failed to generate unpredictable number: %v", err)
	}

	timeNow := time.Now().Format(time.RFC3339)

	cryptogram := model.Cryptogram{
		StatusCard:               model.StatusPending,
		AmountCrypt:              "35.50",
		TimeNowCrypt:             timeNow,
		NumberUnpredictableCrypt: unpredictableNumber,
	}

	result, err := paymentCardCase.Crypto(cryptogram)
	if err != nil {
		t.Fatalf("Crypto returned error: %v", err)
	}

	if result == "" {
		t.Fatal("cryptogram result is empty")
	}

	if len(result) != 32 {
		t.Errorf(
			"cryptogram length: got %d, want 32 hexadecimal characters",
			len(result),
		)
	}

	_, err = hex.DecodeString(result)
	if err != nil {
		t.Errorf("cryptogram is not valid hexadecimal: %v", err)
	}

	t.Logf("Unpredictable Number: %X", unpredictableNumber)
	t.Logf("Cryptogram: %s", result)
}
func TestTlv(t *testing.T) {
	const testKey = "00112233445566778899aabbccddeeff"

	oldEnv, err := os.ReadFile(".env")
	envExisted := err == nil

	err = os.WriteFile(
		".env",
		[]byte("AES_GCM="+testKey+"\n"),
		0600,
	)
	if err != nil {
		t.Fatalf("error creating test .env: %v", err)
	}

	defer func() {
		if envExisted {
			_ = os.WriteFile(".env", oldEnv, 0600)
		} else {
			_ = os.Remove(".env")
		}
	}()

	paymentCardCase := usecases.NewPaymentCardDependencyCase(nil, nil)

	emv := model.EmvTags{
		PAN:                "1234567890123456",
		DateExpiry:         "31/12/27",
		TransactionCountry: "076",
		Pin: model.PIN{
			PIN: "1234",
			CVM: "ONLINE_PIN",
		},
	}

	cryptogram := "070a16b46b4d4144"

	unpredictableNumber := [4]byte{
		0x12,
		0x34,
		0x56,
		0x78,
	}

	result, err := paymentCardCase.Tlv(
		emv,
		cryptogram,
		unpredictableNumber,
	)
	if err != nil {
		t.Fatalf("Tlv returned unexpected error: %v", err)
	}

	// PAN - 5A
	expectedPanTag := []byte{0x5A}

	expectedPanValue := []byte{
		0x12,
		0x34,
		0x56,
		0x78,
		0x90,
		0x12,
		0x34,
		0x56,
	}

	if !bytes.Equal(result.PAN.Tag, expectedPanTag) {
		t.Errorf(
			"PAN tag: got %X, want %X",
			result.PAN.Tag,
			expectedPanTag,
		)
	}

	if !bytes.Equal(result.PAN.Value, expectedPanValue) {
		t.Errorf(
			"PAN value: got %X, want %X",
			result.PAN.Value,
			expectedPanValue,
		)
	}

	if result.PAN.Length != 8 {
		t.Errorf(
			"PAN length: got %d, want 8",
			result.PAN.Length,
		)
	}

	// DateExpiry - 5F24
	expectedExpiryTag := []byte{0x5F, 0x24}
	expectedExpiryValue := []byte{0x27, 0x12, 0x31}

	if !bytes.Equal(result.DateExpiry.Tag, expectedExpiryTag) {
		t.Errorf(
			"Expiry tag: got %X, want %X",
			result.DateExpiry.Tag,
			expectedExpiryTag,
		)
	}

	if !bytes.Equal(result.DateExpiry.Value, expectedExpiryValue) {
		t.Errorf(
			"Expiry value: got %X, want %X",
			result.DateExpiry.Value,
			expectedExpiryValue,
		)
	}

	if result.DateExpiry.Length != 3 {
		t.Errorf(
			"Expiry length: got %d, want 3",
			result.DateExpiry.Length,
		)
	}

	// Country - 9F1A
	expectedCountryTag := []byte{0x9F, 0x1A}
	expectedCountryValue := []byte{0x00, 0x76}

	if !bytes.Equal(
		result.TransactionCountry.Tag,
		expectedCountryTag,
	) {
		t.Errorf(
			"Country tag: got %X, want %X",
			result.TransactionCountry.Tag,
			expectedCountryTag,
		)
	}

	if !bytes.Equal(
		result.TransactionCountry.Value,
		expectedCountryValue,
	) {
		t.Errorf(
			"Country value: got %X, want %X",
			result.TransactionCountry.Value,
			expectedCountryValue,
		)
	}

	if result.TransactionCountry.Length != 2 {
		t.Errorf(
			"Country length: got %d, want 2",
			result.TransactionCountry.Length,
		)
	}

	// Cryptogram - 9F26
	expectedCryptogramTag := []byte{0x9F, 0x26}

	expectedCryptogramValue, err := hex.DecodeString(cryptogram)
	if err != nil {
		t.Fatalf("invalid cryptogram used by test: %v", err)
	}

	if !bytes.Equal(
		result.Cryptogram.Tag,
		expectedCryptogramTag,
	) {
		t.Errorf(
			"Cryptogram tag: got %X, want %X",
			result.Cryptogram.Tag,
			expectedCryptogramTag,
		)
	}

	if !bytes.Equal(
		result.Cryptogram.Value,
		expectedCryptogramValue,
	) {
		t.Errorf(
			"Cryptogram value: got %X, want %X",
			result.Cryptogram.Value,
			expectedCryptogramValue,
		)
	}

	if result.Cryptogram.Length != len(expectedCryptogramValue) {
		t.Errorf(
			"Cryptogram length: got %d, want %d",
			result.Cryptogram.Length,
			len(expectedCryptogramValue),
		)
	}

	// Unpredictable Number - 9F37
	expectedUnpredictableTag := []byte{0x9F, 0x37}

	if !bytes.Equal(
		result.NumberUnpredictable.Tag,
		expectedUnpredictableTag,
	) {
		t.Errorf(
			"Unpredictable number tag: got %X, want %X",
			result.NumberUnpredictable.Tag,
			expectedUnpredictableTag,
		)
	}

	if !bytes.Equal(
		result.NumberUnpredictable.Value,
		unpredictableNumber[:],
	) {
		t.Errorf(
			"Unpredictable number value: got %X, want %X",
			result.NumberUnpredictable.Value,
			unpredictableNumber,
		)
	}

	if result.NumberUnpredictable.Length != 4 {
		t.Errorf(
			"Unpredictable number length: got %d, want 4",
			result.NumberUnpredictable.Length,
		)
	}

	// PIN
	if result.Pin.PIN == "" {
		t.Error("PIN payload is empty")
	}

	if result.Pin.PIN == emv.Pin.PIN {
		t.Error("PIN remained in plaintext")
	}

	if result.Pin.CVM != emv.Pin.CVM {
		t.Errorf(
			"CVM: got %v, want %v",
			result.Pin.CVM,
			emv.Pin.CVM,
		)
	}

	// Invalid PAN length
	invalidPANLength := emv
	invalidPANLength.PAN = "123"

	_, err = paymentCardCase.Tlv(
		invalidPANLength,
		cryptogram,
		unpredictableNumber,
	)

	if err == nil {
		t.Error("expected error for invalid PAN length")
	}

	// Invalid PAN character
	invalidPANCharacter := emv
	invalidPANCharacter.PAN = "12345678901234A6"

	_, err = paymentCardCase.Tlv(
		invalidPANCharacter,
		cryptogram,
		unpredictableNumber,
	)

	if err == nil {
		t.Error("expected error for PAN containing non-numeric character")
	}

	// Invalid country length
	invalidCountryLength := emv
	invalidCountryLength.TransactionCountry = "76"

	_, err = paymentCardCase.Tlv(
		invalidCountryLength,
		cryptogram,
		unpredictableNumber,
	)

	if err == nil {
		t.Error("expected error for invalid country code length")
	}

	// Invalid country character
	invalidCountryCharacter := emv
	invalidCountryCharacter.TransactionCountry = "0A6"

	_, err = paymentCardCase.Tlv(
		invalidCountryCharacter,
		cryptogram,
		unpredictableNumber,
	)

	if err == nil {
		t.Error("expected error for non-numeric country code")
	}

	// Invalid day format
	invalidDay := emv
	invalidDay.DateExpiry = "1/12/27"

	_, err = paymentCardCase.Tlv(
		invalidDay,
		cryptogram,
		unpredictableNumber,
	)

	if err == nil {
		t.Error("expected error for day without exactly 2 digits")
	}

	// Invalid month
	invalidMonth := emv
	invalidMonth.DateExpiry = "31/13/27"

	_, err = paymentCardCase.Tlv(
		invalidMonth,
		cryptogram,
		unpredictableNumber,
	)

	if err == nil {
		t.Error("expected error for invalid month")
	}

	// Invalid year format
	invalidYear := emv
	invalidYear.DateExpiry = "31/12/2027"

	_, err = paymentCardCase.Tlv(
		invalidYear,
		cryptogram,
		unpredictableNumber,
	)

	if err == nil {
		t.Error("expected error for year without exactly 2 digits")
	}

	t.Logf("PAN: %X", result.PAN.Value)
	t.Logf("Expiry: %X", result.DateExpiry.Value)
	t.Logf("Country: %X", result.TransactionCountry.Value)
	t.Logf("Cryptogram: %X", result.Cryptogram.Value)
	t.Logf(
		"Unpredictable Number: %X",
		result.NumberUnpredictable.Value,
	)
	t.Logf("Encrypted PIN payload: %s", result.Pin.PIN)
}
