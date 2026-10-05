package usecases

import (
	brcode "card_machine/internal/brcode_simplify"
	"card_machine/internal/model"
	"card_machine/internal/repository"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
)

type NewCardMachineCase struct {
	repo *repository.Repository
}

func NewNewCardMachineCase(repo *repository.Repository) *NewCardMachineCase {
	return &NewCardMachineCase{
		repo: repo,
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
func (a *NewCardMachineCase) PostSale(amountValue string) (string, error) {
	merchant, err := a.repo.GetAllValuesInCardMachine()
	urlExemplo := "//https.com.br/caminho/123"
	if err != nil {
		return "", err
	}
	qrCode := brcode.DynamicQRParams{
		MerchantName: merchant.PersonNameStorage,
		MerchantCity: merchant.City,
		Amount:       amountValue,
		URL:          urlExemplo,
	}

	payload, err := brcode.BuildDynamicQR(qrCode)

	if err != nil {
		return "", err
	}

	fmt.Print(payload)
	return "", nil
}

// insert value and return a ID for URL
func (a *NewCardMachineCase) InsertValue(value *brcode.DynamicQRParams) (int, error) {
	timeSale := time.Now().Format(time.RFC3339)
	status := model.StatusPending
	var timeApproved *string

	a.repo.InsertValueInTableSale(timeSale, timeApproved, status, value)
	return 0, nil
}

// prepare to amount for save in databse (future)
func (a *NewCardMachineCase) ConvertStringToIntForDatabase(amount string) (int64, error) {
	val, err := strconv.ParseInt(amount, 10, 64)

	if err != nil {
		return 0, err
	}

	return val, nil

}

//============================================ SALES CASES =================================================================================
