package repository

import (
	"card_machine/internal/model"
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/glebarez/go-sqlite"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Create the table CardMachine (the table principal)
// The fun Exec execute a query in engine the database and return a result and a error
func (r *Repository) CreateTableOfCardMachine() (sql.Result, error) {
	query := `CREATE TABLE IF NOT EXISTS bank_machine(
		uuid TEXT PRIMARY KEY,
		person_name_storage VARCHAR(255)  NOT NULL,
		storage_name VARCHAR(255)  NOT NULL,
		person_cpf VARCHAR(255)  NOT NULL,
		city VARCHAR(255)  NOT NULL,
		state VARCHAR(255)  NOT NULL,
		serial_number VARCHAR(255) NOT NULL
	)`
	return r.db.Exec(query)
}

// / Insert a value in the table CardMachine
func (r *Repository) InsertValueInCardMachine(cardMachine *model.CardMachine) (sql.Result, error) {
	query := `INSERT INTO bank_machine (uuid, person_name_storage, storage_name, person_cpf, city, state, serial_number) VALUES( ?, ?, ?, ?, ?, ?, ?)`
	// Insert the values in the table CardMachine
	return r.db.Exec(query, cardMachine.UUID, cardMachine.PersonNameStorage, cardMachine.StorageName, cardMachine.PersonCpf, cardMachine.City, cardMachine.State, cardMachine.SerialNumber) //insert the arguments in the quer and execute the query in the database
}

// Get all values in the thable CardMachine
func (r *Repository) GetAllValuesInCardMachine() (*model.CardMachine, error) {
	query := `SELECT * FROM bank_machine`
	row, err := r.db.Query(query)
	if err != nil {
		fmt.Print(err)
		return nil, err
	}

	defer row.Close() // Close the rows after processing to free up resources

	machine := &model.CardMachine{}

	for row.Next() {
		var cardMachine model.CardMachine
		err := row.Scan(&cardMachine.UUID, &cardMachine.PersonNameStorage, &cardMachine.StorageName, &cardMachine.PersonCpf, &cardMachine.City, &cardMachine.State, &cardMachine.SerialNumber)
		if err != nil {
			fmt.Print(err)
			return nil, err
		}
		machine = &cardMachine
	}
	if err := row.Err(); err != nil {
		fmt.Print(err)
		return nil, err
	}

	return machine, nil
}

func (r *Repository) CpfExists(cpf string) (bool, error) {
	var count int

	//verify if the cpf exist in the table bank_machine
	query := `SELECT COUNT(1) FROM bank_machine WHERE person_cpf = ?`

	//add the value in count
	err := r.db.QueryRow(query, cpf).Scan(&count)
	if err != nil {
		return false, errors.New("error executing query: " + err.Error())
	}

	return count > 0, nil //if count is greater than 0, return true, else return false

}

// =================================================================== CARD MACHINE ===================================================================
// =================================================================== SALES TABLE =====================================================================
// Creat the table sales for have a history
// the id use for the idnetify a sale.
func (r *Repository) CreateTableSale() (sql.Result, error) {
	query := `CREATE TABLE IF NOT EXISTS sales(
		id INTEGER PRIMARY KEY, 
		time_sale VARCHAR(255)  NOT NULL,
		time_aprove VARCHAR(255),
		amount INTEGER NOT NULL,
		real INTEGER NOT NULL,
		status VARCHAR(255)  NOT NULL
	)`
	return r.db.Exec(query)

}

func (r *Repository) InsertValueInTableSale(time_sale string, time_aprove *string, status model.SaleStatus, cents int64, real int64) (sql.Result, int64, error) {
	query := `INSERT INTO sales (time_sale, time_aprove, amount, real, status) VALUES( ?, ?, ?, ?, ?)`
	// Insert the values in the table CardMachine
	result, err := r.db.Exec(query, time_sale, time_aprove, cents, real, status) //insert the arguments in the quer and execute the query in the database
	if err != nil {
		return nil, 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, 0, err
	}
	return result, id, nil
}

func (r *Repository) ValidatePendingSale(id int) (bool, model.SaleStatus, error) {
	var id_sale int
	var status model.SaleStatus

	//verify if the ID exist in the table bank_machine
	query := `SELECT id, status FROM sales WHERE id = ?`

	//add the value in count
	err := r.db.QueryRow(query, id).Scan(&id_sale, &status)
	if err != nil {
		return false, status, errors.New("error executing query: " + err.Error())
	}

	//if the status dont is "pendent", i mean what this sale has already expired
	if status != model.StatusPending {
		return false, status, nil
	}

	return id_sale > 0, status, nil //if count is greater than 0, return true, else return false

}

func (r *Repository) RecoverValues(id int) (int, int, error) {
	var real int
	var amount int

	//verify if the ID exist in the table bank_machine
	query := `SELECT amount, real FROM sales WHERE id = ?`

	err := r.db.QueryRow(query, id).Scan(&real, &amount)
	if err != nil {
		return 0, 0, err
	}

	return real, amount, nil
}

func (r *Repository) PutValueStatus(id int, status model.SaleStatus) (sql.Result, error) {
	query := `UPDATE sales SET status = ?  WHERE id = ?`
	result, err := r.db.Exec(query, status, id)

	if err != nil {
		return result, err
	}

	return result, nil
}
