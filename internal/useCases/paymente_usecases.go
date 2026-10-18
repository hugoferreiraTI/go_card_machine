package usecases

import (
	"card_machine/internal/kafka_config"
	"card_machine/internal/model"
	"card_machine/internal/repository"
	"fmt"
	"os"
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

func (p *NewPaymenteCase) Payment(payer model.Pix, id int) (model.SaleStatus, error) {
	result, statusInDb, err := p.repo.ValidatePendingSale(id)

	if err != nil {
		return model.StatusExpired, err
	}

	if statusInDb != model.StatusPending {
		event := model.PaymentKafka{
			SaleId: id,
			Value:  payer.AmountPix,
			Status: model.StatusExpired,
		}

		p.kafkaCase.ProducerGo(event)
		return model.StatusExpired, nil
	}

	if !result {
		event := model.PaymentKafka{
			SaleId: id,
			Value:  payer.AmountPix,
			Status: model.StatusExpired,
		}

		p.kafkaCase.ProducerGo(event)
		return model.StatusExpired, err
	}

	real, cents, err := p.repo.RecoverValues(id)
	if err != nil {
		return model.StatusReproved, nil
	}

	value_db := fmt.Sprintf("%d.%02d", real, cents)

	if value_db != payer.AmountPix {
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

	time_aprove := string(time.Now().Format(time.RFC3339))

	_, err = p.repo.InserTimeAprove(time_aprove, id)

	if err != nil {
		return model.StatusReproved, err
	}

	timeAprove, err := p.repo.GetTimeArove(id)

	if err != nil {
		return model.StatusReproved, err
	}

	receiver, err := p.repo.GetAllValuesInCardMachine()

	if err != nil {
		return model.StatusReproved, err
	}
	p.taxReceipt(*receiver, id, payer.AmountPix, timeAprove, model.StatusApproved, payer)

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
			status, err := p.Payment(attempt.Pix, id)
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

			event := model.PaymentKafka{
				SaleId: id,
				Value:  "",
				Status: model.StatusExpired,
			}

			p.kafkaCase.ProducerGo(event)

			return model.StatusExpired, nil
		}
	}
}

func (p *NewPaymenteCase) taxReceipt(data model.CardMachine, id int, value string, time string, status model.SaleStatus, payer model.Pix) {
	text := fmt.Sprintf(`--------------------------------
        COMPROVANTE PIX
           APROVADO
--------------------------------

Valor: R$ %s
Data: %s

ID da venda: %d
Status: %s

PAGADOR
Nome: %s
CPF: %s
Instituição: %s

RECEBEDOR
Nome: %s
CPF/CNPJ: %s
Instituição: %s

--------------------------------
Pagamento realizado via Pix
--------------------------------`,
		value, time, id, status, payer.Name, payer.Cpf, payer.Bank, data.PersonNameStorage, data.PersonCpf, data.BankInstituition)
	fmt.Println(text)

	byteText := []byte(text)

	os.WriteFile("comprovante.txt", byteText, 0644)
}
