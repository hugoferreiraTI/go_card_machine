package usecases

import (
	brcode "card_machine/internal/brcode_simplify"
	"card_machine/internal/model"
	"card_machine/internal/repository"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/skip2/go-qrcode"
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
	//urlExemplo := "//https.com.br/caminho/123"
	if err != nil {
		fmt.Print("erro aqui")
		return "", err
	}

	fmt.Print("chamei a func")

	real, cents, err := a.breakTheAmount(amountValue)
	if err != nil {
		return "", err
	}

	id, err := a.InsertValueInTable(cents, real) //save separeted the cents and real
	amountQrCode := fmt.Sprintf("%d.%02d", real, cents)

	if err != nil {
		return "", err
	}

	qrCode := brcode.DynamicQRParams{
		MerchantName: merchant.PersonNameStorage,
		MerchantCity: merchant.City,
		Amount:       amountQrCode,
	}
	url := fmt.Sprintf("https://pix.com.br/caminho/%d", id)

	qrCode.URL = url

	//genereate a qrcode in file.
	payload, err := brcode.BuildDynamicQR(qrCode)

	if err != nil {
		return "", err
	}

	qrcode.WriteFile(payload, qrcode.High, 256, "qr.png")

	return "qrcocode sucess", nil
}

// insert value and return a ID for URL
func (a *NewCardMachineCase) InsertValueInTable(cents, real int64) (int64, error) {
	timeSale := time.Now().Format(time.RFC3339)
	status := model.StatusPending
	var timeApproved *string

	_, id, err := a.repo.InsertValueInTableSale(timeSale, timeApproved, status, cents, real)

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
