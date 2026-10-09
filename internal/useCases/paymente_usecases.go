package usecases

import (
	"card_machine/internal/model"
	"card_machine/internal/repository"
	"fmt"
)

type NewPaymenteCase struct {
	repo *repository.Repository
}

func NewPaymentCase(repo *repository.Repository) *NewPaymenteCase {
	return &NewPaymenteCase{
		repo: repo,
	}
}

func (p *NewPaymenteCase) Payment(value string, id int) (model.SaleStatus, error) {
	result, statusInDb, err := p.repo.ValidatePendingSale(id)

	if err != nil {
		return model.StatusReproved, err
	}

	if statusInDb != model.StatusPending {
		return model.StatusReproved, nil
	}

	if !result {
		return model.StatusReproved, err
	}

	amount, real, err := p.repo.RecoverValues(id)
	if err != nil {
		return model.StatusReproved, err
	}

	value_db := fmt.Sprintf("%d.%02d", real, amount)

	if value_db != value {
		return model.StatusReproved, nil
	}

	p.repo.PutValueStatus(id, model.StatusApproved)
	return model.StatusApproved, nil
}
