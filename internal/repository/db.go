package repository

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

// Open the connection to the database
func DbConnection() (*sql.DB, error, string) {
	db, err := sql.Open("sqlite3", "./card_machine.db")
	if err != nil {
		fmt.Println("Passei aqui")
		log.Fatal(err)
		return nil, err, ""
	}
	fmt.Println("Database connection established successfully.")

	var sqliteVersion string
	err = db.QueryRow("SELECT sqlite_version()").Scan(&sqliteVersion)
	if err != nil {
		log.Fatal(err)
		return nil, err, ""
	}
	fmt.Println("SQLite version:", sqliteVersion)
	return db, nil, sqliteVersion
}
