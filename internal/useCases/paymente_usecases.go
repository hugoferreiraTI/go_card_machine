package usecases

import (
	"card_machine/internal/model"
	"card_machine/internal/repository"
	"fmt"
	"time"
)

type NewPaymenteCase struct {
	repo *repository.Repository
}

func NewPaymentCase(repo *repository.Repository) *NewPaymenteCase {
	return &NewPaymenteCase{
		repo: repo,
	}
}

func (p *NewPaymenteCase) Payment(value string, id int64) (model.SaleStatus, error) {
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

func (p *NewPaymenteCase) WaitPaytament(paymentChan <-chan PaymentAttempt, id int64) (model.SaleStatus, error) {
	//recive a timer in minutes
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()

	//loop infinite what recive a movement of channel;
	for {
		select {
		case attempt := <-paymentChan:
			status, err := p.Payment(attempt.Pix.AmountPix, id)
			if err != nil {
				fmt.Print("pagamento reprovado")
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
			p.repo.PutValueStatus(id, model.StatusReproved)
			return model.StatusReproved, nil
		}
	}
}
