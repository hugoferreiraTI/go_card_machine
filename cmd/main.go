package main

import (
	"card_machine/documents"
	"card_machine/internal/client"
	"card_machine/internal/handler"
	"card_machine/internal/kafka_config"
	"card_machine/internal/repository"
	usecases "card_machine/internal/useCases"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	// initialize the database connection
	dbConnection, err, version := repository.DbConnection()
	fmt.Println("Version SQLIte: ", version)

	if err != nil {
		panic(err)
	}
	defer dbConnection.Close()

	// Kafka
	produce, _ := kafka_config.NewKafkaProducer()

	consumer, err := kafka_config.NewKafkaConsumer()
	if err != nil {
		panic(err)
	}

	consumerCase := kafka_config.NewConsumerDependency(consumer)
	go consumerCase.ConsumerGo()

	kafkaProduceCase := kafka_config.NewProducerCaseDependency(produce)

	// PSP Client
	pspClient := &client.PSPClient{
		BaseURL: "http://localhost:8080",
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	// Repository
	authRepository := repository.NewRepository(dbConnection)

	// Usecases
	authPayment := usecases.NewPaymentCase(
		authRepository,
		kafkaProduceCase,
	)

	authPaymentCard := usecases.NewPaymentCardDependencyCase(
		authRepository,
		kafkaProduceCase,
		pspClient,
	)

	authUseCase := usecases.NewCardMachineCaseDependency(
		authRepository,
		authPayment,
		authPaymentCard, // <-- FALTAVA ISSO
		kafkaProduceCase,
	)

	// Handler
	authHandler := handler.NewHandlerCase(
		authUseCase,
		authPayment,
		authPaymentCard,
	)

	// Gin
	server := gin.Default()

	documents.RegisterRoutes(server)

	authRepository.CreateTableOfCardMachine()
	authRepository.CreateTableSale()

	server.POST("/card_machine", authHandler.InsertValueInCardMachine)
	server.POST("/Sale", authHandler.PostSale)
	server.POST("/Pay/pix", authHandler.PaymentPix)
	server.POST("/Pay/card", authHandler.PaymentCard)

	server.Run(":8081")
}
