package usecases

import (
	"card_machine/internal/model"
	"card_machine/internal/repository"
	"database/sql"
	"errors"

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
