package main

import (
	"card_machine/internal/handler"
	"card_machine/internal/repository"
	usecases "card_machine/internal/useCases"
	"fmt"

	"github.com/gin-gonic/gin"
)

func main() {
	//initialize the database connection
	dbConnection, err, version := repository.DbConnection()
	fmt.Println("Version SQLIte: ", version)

	if err != nil {
		panic(err)
	}
	defer dbConnection.Close()

	//injection of dependencies
	authRepository := repository.NewRepository(dbConnection)
	authUseCase := usecases.NewNewCardMachineCase(authRepository)
	authHandler := handler.NewHandlerCase(authUseCase)

	//initialize the Gin router
	server := gin.Default()

	//create the table
	authRepository.CreateTableOfCardMachine()

	server.POST("/card_machine", authHandler.InsertValueInCardMachine) //initialize the card machine first.
	server.Run(":8080")
}
