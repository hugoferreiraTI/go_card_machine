package usecases

import (
	"card_machine/internal/kafka_config"
	"card_machine/internal/model"
	"card_machine/internal/repository"
	"fmt"
	"time"
)

type NewPaymenteCase struct {
	repo      *repository.Repository
	kafkaCase *kafka_config.NewProducerCase
}

func NewPaymentCase(repo *repository.Repository, kafkaCase *kafka_config.NewProducerCase) *NewPaymenteCase {
	return &NewPaymenteCase{
		repo:      repo,
		kafkaCase: kafkaCase,
	}
}

func (p *NewPaymenteCase) Payment(value string, id int) (model.SaleStatus, error) {
	result, statusInDb, err := p.repo.ValidatePendingSale(id)

	if err != nil {
		return model.StatusExpired, err
	}

	if statusInDb != model.StatusPending {
		return model.StatusExpired, nil
	}

	if !result {
		return model.StatusExpired, err
	}

	real, cents, err := p.repo.RecoverValues(id)
	if err != nil {
		return model.StatusReproved, nil
	}

	value_db := fmt.Sprintf("%d.%02d", real, cents)

	if value_db != value {
		event := model.PaymentKafka{
			SaleId: id,
			Value:  value_db,
			Status: model.StatusReproved,
		}

		p.kafkaCase.ProducerGo(event)
		return model.StatusReproved, nil
	}

	_, err = p.repo.PutValueStatus(id, model.StatusApproved)
	if err != nil {
		return model.StatusExpired, err
	}

	event := model.PaymentKafka{
		SaleId: id,
		Value:  value_db,
		Status: model.StatusApproved,
	}

	p.kafkaCase.ProducerGo(event)

	return model.StatusApproved, nil
}

func (p *NewPaymenteCase) WaitPaytament(paymentChan <-chan PaymentAttempt, id int) (model.SaleStatus, error) {
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
			_, err := p.repo.PutValueStatus(id, model.StatusExpired)
			if err != nil {

				return model.StatusExpired, err
			}
			return model.StatusExpired, nil
		}
	}
}
