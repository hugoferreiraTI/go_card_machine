package main

import (
	"card_machine/documents"
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

	produce, _ := kafka_config.NewKafkaProducer() //initializer the producer

	consumer, err := kafka_config.NewKafkaConsumer()

	if err != nil {
		panic(err)
	} //initialize the consumer kafka

	consumerCase := kafka_config.NewConsumerDependency(consumer)

	go consumerCase.ConsumerGo()

	kafkaProduceCase := kafka_config.NewProducerCaseDependency(produce)

	//injection of dependencies
	authRepository := repository.NewRepository(dbConnection)
	authPayment := usecases.NewPaymentCase(authRepository, kafkaProduceCase)
	authUseCase := usecases.NewCardMachineCaseDependency(authRepository, authPayment, kafkaProduceCase)
	authHandler := handler.NewHandlerCase(authUseCase)

	//initialize the Gin router
	server := gin.Default()
	documents.RegisterRoutes(server)

	//create the table
	authRepository.CreateTableOfCardMachine()
	authRepository.CreateTableSale()

	server.POST("/card_machine", authHandler.InsertValueInCardMachine) //initialize the card machine first.
	server.POST("/Sale", authHandler.PostSales)
	server.POST("/Pay", authHandler.Payment)
	server.Run(":8080")
}
