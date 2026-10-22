package usecases

import (
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

	"github.com/google/uuid"
	"github.com/hugoferreiraTI/AES_CMAC_Go/cmac"
	"github.com/joho/godotenv"
)

type NewPaymentCardCase struct {
	repo        *repository.Repository
	kafkaCase   *kafka_config.NewProducerCase
	PaymentChan chan PaymentCard
}

type PaymentCard struct {
	Tag      model.EmvTags
	Response chan model.SaleStatus
}

func NewPaymentCardDependencyCase(repo *repository.Repository, kafkaCase *kafka_config.NewProducerCase) *NewPaymentCardCase {
	return &NewPaymentCardCase{
		repo:        repo,
		kafkaCase:   kafkaCase,
		PaymentChan: make(chan PaymentCard),
	}
}

func (c *NewPaymentCardCase) ProcessSaleCard(real, cents int64, id int) (model.SaleStatus, error) {
	amount := fmt.Sprintf("%d.%02d", real, cents)
	time_now := time.Now().Format(time.RFC3339)
	time := string(time_now)

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

	uuidCrypto := uuid.New()

	cryptogram := model.Cryptogram{
		StatusCard:               statusInDb,
		AmountCrypt:              amount,
		TimeNowCrypt:             time,
		NumberUnpredictableCrypt: uuidCrypto.String(),
	}

	cryptogramResult, err := c.Crypto(cryptogram)
	if err != nil {
		return model.StatusReproved, err
	}

	unpredictableNumber, err := c.generateUnpredictableNumber()
	if err != nil {
		return model.StatusReproved, err
	}

	fmt.Print(unpredictableNumber)

	fmt.Print(cryptogramResult)

	return model.StatusApproved, nil
}

func (c *NewPaymentCardCase) PaymentCard(tags model.EmvTags, id int) (model.SaleStatus, error) {
	return model.StatusApproved, nil
}

func (c *NewPaymentCardCase) WaitPaymentCard(paymentChan <-chan PaymentCard, id int) (model.SaleStatus, error) {
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()

	//loop infinite what recive a movement of channel;
	for {
		select {
		case attempt := <-paymentChan:
			status, err := c.PaymentCard(attempt.Tag, id)
			if err != nil {
				fmt.Print("pagamento reprovado")
				attempt.Response <- status
				return status, err
			}

			//if status == model.StatusReproved {
			//	fmt.Print(model.StatusReproved)
			//} use for verify the result in test.
			attempt.Response <- status
			if status == model.StatusApproved {
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

func (c *NewPaymentCardCase) Tlv(emv model.EmvTags, cryptogram string, unpredictableNumber []byte) (model.EmvTlv, error) {
	//format a date expirity em default tag
	var day string
	var mounth string
	var year string

	//format a slice of pan
	var pan []byte

	parts := strings.Split(emv.DateExpiry, "/")

	if len(parts) != 3 {
		return model.EmvTlv{}, errors.New("Value date invalide.")
	}

	day = parts[0]
	mounth = parts[1]
	year = parts[2]

	//Creat the tag 5F24 in TLV mode
	dateOfExpirity := fmt.Sprintf("%s%s%s", year, mounth, day)
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

	valueCountryCode := emv.TransactionCountry

	if len(valueCountryCode) == 3 {
		valueCountryCode = "0" + valueCountryCode
	}

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
			Value:  unpredictableNumber,
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

func (c *NewPaymentCardCase) generateUnpredictableNumber() ([4]byte, error) {
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
