package test_sql

import (
	"card_machine/internal/model"
	"card_machine/internal/repository"
	usecases "card_machine/internal/useCases"
	"fmt"
	"testing"
)

func TestPutStatus(t *testing.T) {
	value := "100.50"
	var id int
	id = 1

	// Call the DbConnection function to test the database connection
	db, err, _ := repository.DbConnection()
	if err != nil {
		fmt.Print(err)
		t.Fatal(err)
		return
	}

	//Initialize the repository with dependency
	repo := repository.NewRepository(db)
	Paycase := usecases.NewPaymentCase(repo)

	status, err := Paycase.Payment(value, id)

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
	go func() {
		amount <- usecases.PaymentAttempt{
			Pix: model.Pix{
				AmountPix: "200.00",
			},
		}
	}()
	var id int
	id = 4

	// Call the DbConnection function to test the database connection
	db, err, _ := repository.DbConnection()
	if err != nil {
		fmt.Print(err)
		t.Fatal(err)
		return
	}

	//Initialize the repository with dependency
	repo := repository.NewRepository(db)
	paycase := usecases.NewPaymentCase(repo)

	status, err := paycase.WaitPaytament(amount, id)

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
