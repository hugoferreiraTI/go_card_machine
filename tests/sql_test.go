package test_sql

import (
	"card_machine/internal/model"
	"card_machine/internal/repository"
	"fmt"
	"testing"

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
