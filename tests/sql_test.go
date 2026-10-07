package test_sql

import (
	"card_machine/internal/model"
	"card_machine/internal/repository"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestConnection1(t *testing.T) {
	// Call the DbConnection function to test the database connection
	db, err, version := repository.DbConnection()

	if err != nil {
		fmt.Println("Error in connection", err)
		return
	}

	defer db.Close()
	fmt.Println("Connection successful")
	fmt.Println("Version: ", version)

}

// Test a function for create table in the database
func TestCreateTableOfCardMachine1(t *testing.T) {
	// Call the DbConnection function to test the database connection
	db, err, _ := repository.DbConnection()
	if err != nil {
		fmt.Println("Error in connection", err)
		return
	}

	//Initialize the repository with dependency
	repo := repository.NewRepository(db)

	//Call the func for create the table
	log, err := repo.CreateTableOfCardMachine()
	if err != nil {
		fmt.Println("Error creating table:", err)
		return
	}

	fmt.Println("Table created successfully:", log)
}

func TestCreateTableSale(t *testing.T) {
	// Call the DbConnection function to test the database connection
	db, err, _ := repository.DbConnection()
	if err != nil {
		fmt.Println("Error in connection", err)
		return
	}

	//Initialize the repository with dependency
	repo := repository.NewRepository(db)

	//Call the func for create the table
	log, err := repo.CreateTableSale()
	if err != nil {
		fmt.Println("Error creating table:", err)
		return
	}

	fmt.Println("Table created successfully:", log)
}

func TestInsertValueInTableSale(t *testing.T) {
	//variables dependencys
	time_sale := time.Now()
	var time_approved *string
	var real int64
	var cents int64
	//initialize in pendent for test
	status := model.StatusPending

	// Call the DbConnection function to test the database connection
	db, err, _ := repository.DbConnection()
	if err != nil {
		fmt.Println("Error in connection", err)
		return
	}

	real = 100
	cents = 50

	//Initialize the repository with dependency
	repo := repository.NewRepository(db)

	result, id, err := repo.InsertValueInTableSale(time_sale.String(), time_approved, status, cents, real)

	if err != nil {
		fmt.Print(err)
	}
	fmt.Print(result)
	fmt.Print(id)
}

func TestInsertValueInTableCardMachine1(t *testing.T) {
	// Call the DbConnection function to test the database connection
	db, err, _ := repository.DbConnection()
	if err != nil {
		fmt.Println("Error in connection", err)
		return
	}

	//Initialize the repository with dependency
	repo := repository.NewRepository(db)

	testValue := model.CardMachine{
		UUID:              uuid.New().String(),
		PersonNameStorage: "Test Machine TESTANDO",
		StorageName:       "Test Storage",
		PersonCpf:         "123.456.789-00",
		City:              "Test City",
		State:             "Test State",
		SerialNumber:      "123456789101010101010",
	}

	fmt.Print(testValue.UUID)

	log, err := repo.InsertValueInCardMachine(&testValue)
	if err != nil {
		fmt.Print(err)
		return
	}

	fmt.Print(log)

}

func TestGetAllValuesInTableCardMachine(t *testing.T) {
	// Call the DbConnection function to test the database connection
	db, err, _ := repository.DbConnection()
	if err != nil {
		fmt.Println("Error in connection", err)
		return
	}

	//Initialize the repository with dependency
	repo := repository.NewRepository(db)

	log, err := repo.GetAllValuesInCardMachine()
	if err != nil {
		fmt.Print(err)
		return
	}

	fmt.Print(log)
}
