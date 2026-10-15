package main

import (
	"card_machine/internal/handler"
	"card_machine/internal/kafka_config"
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

	produce, _ := kafka_config.NewKafkaProducer()
	kafkaCase := kafka_config.NewKafkaCaseDependency(produce)

	//injection of dependencies
	authRepository := repository.NewRepository(dbConnection)
	authPayment := usecases.NewPaymentCase(authRepository)
	authUseCase := usecases.NewNewCardMachineCase(authRepository, authPayment, kafkaCase)
	authHandler := handler.NewHandlerCase(authUseCase)

	//initialize the Gin router
	server := gin.Default()

	//create the table
	authRepository.CreateTableOfCardMachine()
	authRepository.CreateTableSale()

	server.POST("/card_machine", authHandler.InsertValueInCardMachine) //initialize the card machine first.
	server.POST("/Sale", authHandler.PostSales)
	server.POST("/Pay", authHandler.Payment)
	server.Run(":8080")
}
