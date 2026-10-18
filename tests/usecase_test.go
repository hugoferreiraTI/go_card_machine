package test_sql

import (
	"card_machine/internal/handler"
	"card_machine/internal/kafka_config"
	"card_machine/internal/model"
	"card_machine/internal/repository"
	usecases "card_machine/internal/useCases"
	"fmt"
	"os"
	"testing"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

func TestPutStatus(t *testing.T) {
	value := model.Pix{
		AmountPix: "100.10",
		Cpf:       "12345678912",
		Name:      "diego",
		Bank:      "nubank",
	}
	var id int
	id = 1

	// Call the DbConnection function to test the database connection
	db, err, _ := repository.DbConnection()
	if err != nil {
		fmt.Print(err)
		t.Fatal(err)
		return
	}
	produce, err := kafka.NewProducer(&kafka.ConfigMap{
		// User-specific properties that you must set
		"bootstrap.servers": "localhost:9092", //run in docker local
		// Fixed properties
		"acks": "all"})

	if err != nil {
		fmt.Printf("Failed to create producer: %s", err)
		os.Exit(1)
	}

	//Initialize the repository with dependency
	repo := repository.NewRepository(db)
	kafkaCase := kafka_config.NewProducerCaseDependency(produce)
	paymentCase := usecases.NewPaymentCase(repo, kafkaCase)
	authCase := usecases.NewCardMachineCaseDependency(repo, paymentCase, kafkaCase)
	_ = handler.NewHandlerCase(authCase)

	status, err := paymentCase.Payment(value, id)

	if err != nil {
		fmt.Print(status)
		fmt.Print(err)
		t.Fatal(err)
		return
	}

	fmt.Print(err)
	fmt.Print(status)

}

func TestPayment(t *testing.T) {
	amount := make(chan usecases.PaymentAttempt)
	responseChan := make(chan model.SaleStatus)

	go func() {
		amount <- usecases.PaymentAttempt{
			Pix: model.Pix{
				AmountPix: "200.00",
				Cpf:       "12345678912",
				Name:      "Hugo",
				Bank:      "nubank",
			},
			Response: responseChan,
		}
	}()
	id := 4

	// Call the DbConnection function to test the database connection
	db, err, _ := repository.DbConnection()
	if err != nil {
		fmt.Print(err)
		t.Fatal(err)
		return
	}

	produce, err := kafka.NewProducer(&kafka.ConfigMap{
		// User-specific properties that you must set
		"bootstrap.servers": "localhost:9092", //run in docker local
		// Fixed properties
		"acks": "all"})

	if err != nil {
		fmt.Printf("Failed to create producer: %s", err)
		os.Exit(1)
	}

	//Initialize the repository with dependency
	repo := repository.NewRepository(db)
	kafkaCase := kafka_config.NewProducerCaseDependency(produce)
	paymentCase := usecases.NewPaymentCase(repo, kafkaCase)
	authCase := usecases.NewCardMachineCaseDependency(repo, paymentCase, kafkaCase)
	_ = handler.NewHandlerCase(authCase)

	status, err := paymentCase.WaitPaytament(amount, id)

	if err != nil {
		fmt.Print("paymente repproved")
		return
	}

	if status == model.StatusApproved {
		fmt.Print("payment approved")
		return
	}

	fmt.Print(status)

}
