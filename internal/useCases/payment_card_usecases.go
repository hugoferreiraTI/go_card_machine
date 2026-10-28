package usecases

import (
	"card_machine/internal/client"
	"card_machine/internal/kafka_config"
	"card_machine/internal/model"
	"card_machine/internal/repository"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hugoferreiraTI/AES_CMAC_Go/cmac"
	"github.com/joho/godotenv"
)

type NewPaymentCardCase struct {
	repo        *repository.Repository
	kafkaCase   *kafka_config.NewProducerCase
	PaymentChan chan PaymentCard
	PSPClient   *client.PSPClient
}

type PaymentCard struct {
	Tag      model.EmvTags
	Response chan model.SaleStatus
}

func NewPaymentCardDependencyCase(repo *repository.Repository, kafkaCase *kafka_config.NewProducerCase, PSPClient *client.PSPClient) *NewPaymentCardCase {
	return &NewPaymentCardCase{
		repo:        repo,
		kafkaCase:   kafkaCase,
		PSPClient:   PSPClient,
		PaymentChan: make(chan PaymentCard),
	}
}

func (c *NewPaymentCardCase) ProcessSaleCard(real, cents int64, id int) (model.SaleStatus, error) {
	amount := fmt.Sprintf("%d.%02d", real, cents)
	timeNow := time.Now().Format(time.RFC3339)
	time := string(timeNow)

	result, statusInDb, err := c.repo.ValidatePendingSale(id)

	if err != nil {
		return model.StatusExpired, err
	}

	if !result {
		return model.StatusReproved, nil
	}

	if statusInDb != model.StatusPending {
		return statusInDb, nil
	}

	saleData := model.CardSale{
		SaleId:      id,
		Amount:      amount,
		PaymentType: model.Debit,
		TimeNow:     time,
		Status:      model.StatusPending,
	}
	go c.WaitPaymentCard(c.PaymentChan, saleData, id)

	return model.StatusPending, nil
}

func (c *NewPaymentCardCase) PaymentSaleCard(tags model.EmvTags, saleData model.CardSale, id int) (model.AuthorizationResponse, error) {
	numberUnpredictable, err := c.GenerateUnpredictableNumber()
	if err != nil {
		return model.AuthorizationResponse{}, err
	}
	crypto := model.Cryptogram{
		StatusCard:               saleData.Status,
		AmountCrypt:              saleData.Amount,
		TimeNowCrypt:             saleData.TimeNow,
		NumberUnpredictableCrypt: numberUnpredictable,
	}

	cryptogram, err := c.Crypto(crypto)
	if err != nil {
		return model.AuthorizationResponse{}, err
	}

	emvTlv, err := c.Tlv(tags, cryptogram, numberUnpredictable)
	if err != nil {
		return model.AuthorizationResponse{}, err
	}

	cardPayment := model.CardPayment{
		SaleId:      id,
		TimeNow:     saleData.TimeNow,
		Amount:      saleData.Amount,
		Status:      saleData.Status,
		PaymentType: model.Debit,
		Emv:         emvTlv,
	}

	resp, err := c.PSPClient.PSP(cardPayment)
	if err != nil {
		return model.AuthorizationResponse{}, err
	}
	return resp, nil
}

///authorize/card

func (c *NewPaymentCardCase) WaitPaymentCard(paymentChan <-chan PaymentCard, saleData model.CardSale, id int) (model.SaleStatus, error) {
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()

	//loop infinite what recive a movement of channel;
	for {
		select {
		case attempt := <-paymentChan:
			resp, err := c.PaymentSaleCard(attempt.Tag, saleData, id)
			if err != nil {
				fmt.Print("pagamento reprovado")
				attempt.Response <- resp.Status
				return resp.Status, err
			}

			//if status == model.StatusReproved {
			//	fmt.Print(model.StatusReproved)
			//} use for verify the result in test.
			attempt.Response <- resp.Status
			if resp.Status == model.StatusApproved {
				return model.StatusApproved, nil
			}
			//if the time concluide are stop the loop (expired)
		case <-timer.C: //this case, the C its is a CHANNEL
			_, err := c.repo.PutValueStatus(id, model.StatusExpired)
			if err != nil {

				return model.StatusExpired, err
			}

			event := model.PaymentKafka{
				SaleId: id,
				Value:  "",
				Status: model.StatusExpired,
			}

			c.kafkaCase.ProducerGo(event)

			return model.StatusExpired, nil
		}
	}

}

func (c *NewPaymentCardCase) Crypto(cryptogram model.Cryptogram) (string, error) {
	cryptoString := fmt.Sprintf(
		"%s|%s|%s|%s",
		cryptogram.StatusCard,
		cryptogram.AmountCrypt,
		cryptogram.TimeNowCrypt,
		cryptogram.NumberUnpredictableCrypt,
	)
	err := godotenv.Load(".env")
	if err != nil {
		return "", err
	}

	keyHex := os.Getenv("AES_CMAC_KEY")

	cryptogramResult, err := cmac.Generate(keyHex, cryptoString)
	if err != nil {
		return "", err
	}

	return cryptogramResult, nil

}

func (c *NewPaymentCardCase) Tlv(emv model.EmvTags, cryptogram string, unpredictableNumber [4]byte) (model.EmvTlv, error) {
	//format a date expirity em default tag
	var day string
	var month string
	var year string

	//format a slice of pan
	var pan []byte

	parts := strings.Split(emv.DateExpiry, "/")

	if len(parts) != 3 {
		return model.EmvTlv{}, errors.New("Value date invalide.")
	}

	day = parts[0]
	if len(day) != 2 {
		return model.EmvTlv{}, errors.New("day must contain exactly 2 digits")
	}
	verifyDay, err := strconv.Atoi(day)
	if err != nil {
		return model.EmvTlv{}, err
	}
	if verifyDay > 31 || 1 > verifyDay {
		return model.EmvTlv{}, errors.New("The days is invalid")
	}
	month = parts[1]
	verifyMonth, err := strconv.Atoi(month)
	if len(month) != 2 {
		return model.EmvTlv{}, errors.New("month must contain exactly 2 digits")
	}
	if err != nil {
		return model.EmvTlv{}, err
	}
	if verifyMonth > 12 || 1 > verifyMonth {
		return model.EmvTlv{}, errors.New("The month is invalid")
	}
	year = parts[2]
	if len(year) != 2 {
		return model.EmvTlv{}, errors.New("year must contain exactly 2 digits")
	}
	verifyYears, err := strconv.Atoi(year)
	if err != nil {
		return model.EmvTlv{}, err
	}
	if verifyYears > 31 || verifyYears < 21 {
		return model.EmvTlv{}, errors.New("The yeras is invalid")
	}

	//Creat the tag 5F24 in TLV mode
	dateOfExpirity := fmt.Sprintf("%s%s%s", year, month, day)
	dateOfExpirityBytes, err := hex.DecodeString(dateOfExpirity)
	tagStringDateOfExpirity := "5F24"
	if err != nil {
		return model.EmvTlv{}, err
	}

	tagDateOfExpirity, err := hex.DecodeString(tagStringDateOfExpirity)
	if err != nil {
		return model.EmvTlv{}, err
	}

	dateOfExpirityBytesLenght := len(dateOfExpirityBytes)

	//Creat the PAN in TLV mode
	tagPanString := "5A"
	tagPan, err := hex.DecodeString(tagPanString)
	if err != nil {
		return model.EmvTlv{}, err
	}
	//verify the pan
	err = c.verifyThePan(emv.PAN)
	if err != nil {
		return model.EmvTlv{}, err
	}
	//Compact the values in two pars.
	if len(emv.PAN)%2 != 0 {
		emv.PAN += "F"
	}
	for i := 0; i < len(emv.PAN); i += 2 {
		pair := emv.PAN[i : i+2]
		pairBytes, err := strconv.ParseUint(pair, 16, 8)
		if err != nil {
			return model.EmvTlv{}, err
		}
		pan = append(pan, byte(pairBytes))

	}

	panLength := len(pan)

	//Creat the transaction couder from country
	tagCountryString := "9F1A"

	tagCountry, err := hex.DecodeString(tagCountryString)
	if err != nil {
		return model.EmvTlv{}, err
	}

	err = c.verifyTheCountryCode(emv.TransactionCountry)
	if err != nil {
		return model.EmvTlv{}, err
	}

	valueCountryCode := "0" + emv.TransactionCountry
	valueCountry, err := hex.DecodeString(valueCountryCode)
	if err != nil {
		return model.EmvTlv{}, err
	}

	countryLen := len(valueCountry)

	//Tag cryptogram
	tagCryptogramString := "9F26"
	tagCryptogram, err := hex.DecodeString(tagCryptogramString)
	if err != nil {
		return model.EmvTlv{}, err
	}
	cryptogramValue, err := hex.DecodeString(cryptogram)
	if err != nil {
		return model.EmvTlv{}, err
	}
	lenCryptogram := len(cryptogramValue)

	//Tag unpredicatableNumber
	tagUnpredictableNumberString := "9F37"
	tagUnpredictableNumber, err := hex.DecodeString(tagUnpredictableNumberString)
	if err != nil {
		return model.EmvTlv{}, err
	}
	unpredictableNumberLen := len(unpredictableNumber)

	//Tag PIN
	pin := emv.Pin.PIN
	cvm := emv.Pin.CVM
	pinPayload, err := c.aesGcm(pin)
	if err != nil {
		return model.EmvTlv{}, err
	}

	emvTlv := model.EmvTlv{
		PAN: model.TLV{
			Tag:    tagPan,
			Value:  pan,
			Length: panLength,
		},
		Pin: model.PIN{
			PIN: pinPayload,
			CVM: cvm,
		},

		Cryptogram: model.TLV{
			Tag:    tagCryptogram,
			Value:  cryptogramValue,
			Length: lenCryptogram,
		},
		NumberUnpredictable: model.TLV{
			Tag:    tagUnpredictableNumber,
			Value:  unpredictableNumber[:],
			Length: unpredictableNumberLen,
		},
		TransactionCountry: model.TLV{
			Tag:    tagCountry,
			Value:  valueCountry,
			Length: countryLen,
		},
		DateExpiry: model.TLV{
			Tag:    tagDateOfExpirity,
			Value:  dateOfExpirityBytes,
			Length: dateOfExpirityBytesLenght,
		},
	}

	return emvTlv, nil
}

func (c *NewPaymentCardCase) GenerateUnpredictableNumber() ([4]byte, error) {
	var value [4]byte

	_, err := rand.Read(value[:])
	if err != nil {
		return [4]byte{}, err
	}

	return value, nil
}

func (c *NewPaymentCardCase) aesGcm(pin string) (string, error) {
	if err := godotenv.Load(".env"); err != nil {
		return "", err
	}

	keyHex := os.Getenv("AES_GCM")

	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())

	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(pin), nil)

	payload := append(nonce, ciphertext...)

	return hex.EncodeToString(payload), nil
}

func (c *NewPaymentCardCase) verifyThePan(pan string) error {
	parts := pan

	if len(parts) < 12 || len(parts) > 19 {
		return errors.New("the value in PAN is incorrect")
	}

	for _, char := range parts {
		if char < '0' || char > '9' {
			return errors.New("PAN must contain only numbers")
		}
	}

	return nil
}

func (c *NewPaymentCardCase) verifyTheCountryCode(countryCode string) error {
	if len(countryCode) != 3 {
		return errors.New("country code must contain exactly 3 digits")
	}

	for _, char := range countryCode {
		if char < '0' || char > '9' {
			return errors.New("country code must contain only numbers")
		}
	}

	return nil
}
