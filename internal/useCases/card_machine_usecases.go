package usecases

import (
	"card_machine/internal/kafka_config"
	"card_machine/internal/model"
	"card_machine/internal/repository"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type NewCardMachineCase struct {
	repo            *repository.Repository
	paymentPixCase  *NewPaymenteCase
	paymentCardCase *NewPaymentCardCase
	kafkaCase       *kafka_config.NewProducerCase
}

func NewCardMachineCaseDependency(
	repo *repository.Repository,
	paymentCase *NewPaymenteCase,
	kafkaCase *kafka_config.NewProducerCase,
) *NewCardMachineCase {
	return &NewCardMachineCase{
		repo:           repo,
		paymentPixCase: paymentCase,
		kafkaCase:      kafkaCase,
	}
}

func (a *NewCardMachineCase) InsertValueInCardMachine(cardMachine *model.CardMachine) (sql.Result, error) {
	values := cardMachine
	result, err := a.repo.CpfExists(values.PersonCpf)

	if err != nil {
		return nil, err
	}

	if result {
		return nil, errors.New("Cpf exists")
	}

	values.UUID = uuid.New().String()         // Generate a new UUID for the card machine
	values.SerialNumber = uuid.New().String() // Generate a new UUID for the serial number

	return a.repo.InsertValueInCardMachine(values) //call the sql what execu the comand in the database
}

// ============================================ SALES CASES =================================================================================
func (a *NewCardMachineCase) PostSale(sale model.Sale) (string, error) {
	status, _, err := a.repo.VerifyTheStatus()

	if err != nil {
		return "", err
	}

	if status == model.StatusPending {
		return "", errors.New("The last sale is active.")
	}

	real, cents, err := a.breakTheAmount(sale.Amount)
	if err != nil {
		return "", err
	}

	if sale.PayType == model.PaymentPix {

		enumPix := model.PaymentPix

		id, err := a.InsertValueInTable(cents, real, enumPix) //save separeted the cents and real

		if err != nil {
			return "", err
		}

		result, err := a.paymentPixCase.ProcessPixSale(real, cents, id)
		if err != nil {
			return "", err
		}

		return result, nil

	}

	if sale.PayType == model.PaymentCard {
		enumCard := model.PaymentCard

		id, err := a.InsertValueInTable(cents, real, enumCard) //save separeted the cents and real

		if err != nil {
			return "", err
		}

		result, err := a.paymentCardCase.ProcessSaleCard(real, cents, id)
		if err != nil {
			return "", err
		}

		resultString := string(result)

		return resultString, nil

	}

	return "", nil
}

// insert value and return a ID for URL
func (a *NewCardMachineCase) InsertValueInTable(cents, real int64, typePayment model.PaymentType) (int, error) {
	timeSale := time.Now().Format(time.RFC3339)
	status := model.StatusPending
	var timeApproved *string

	_, id, err := a.repo.InsertValueInTableSale(timeSale, timeApproved, status, cents, real, typePayment)

	if err != nil {
		return 0, err
	}

	return id, nil
}

func (a *NewCardMachineCase) breakTheAmount(amount string) (real, cents int64, err error) {
	parts := strings.Split(amount, ".") //recive a string, and breaking in parts where have ","
	var real_text string
	var cents_text string

	if len(parts) == 1 {
		//dont have cents
		real_text = parts[0]
		cents_text = "00"
	}

	if len(parts) == 2 {
		real_text = parts[0]
		cents_text = parts[1]
	}
	if len(parts) >= 3 {
		err = errors.New("invalid value")
		return 0, 0, err
	}

	if len(cents_text) == 1 {
		cents_text += "0"
	} //convert one decimal digit to cents. Example: "10.5" -> "10.50"

	if len(cents_text) > 2 {
		return 0, 0, errors.New("invalid cents")
	} //reject values with more than two decimal digits. Example: "10.500"

	real_value, err := strconv.ParseInt(real_text, 10, 64)
	if err != nil {
		return 0, 0, err
	}

	cents_value, err := strconv.ParseInt(cents_text, 10, 64)
	if err != nil {
		return 0, 0, err
	}

	return real_value, cents_value, nil

}

//============================================ SALES CASES =================================================================================
