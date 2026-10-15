package test_sql

import (
	"bytes"
	"card_machine/internal/handler"
	"card_machine/internal/kafka_config"
	"card_machine/internal/repository"
	usecases "card_machine/internal/useCases"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/gin-gonic/gin"
)

func TestHttpInsertValue(t *testing.T) {
	gin.SetMode(gin.TestMode) //Test mode for dont poluid my console with logs
	// Call the DbConnection function to test the database connection
	db, err, _ := repository.DbConnection()
	if err != nil {
		fmt.Println("Error in connection", err)
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
	paymentCase := usecases.NewPaymentCase(repo)
	kafkaCase := kafka_config.NewKafkaCaseDependency(produce)
	authCase := usecases.NewNewCardMachineCase(repo, paymentCase, kafkaCase)
	handlerCase := handler.NewHandlerCase(authCase)
	// Create the route
	router := gin.Default()
	router.POST("/card_machine", handlerCase.InsertValueInCardMachine)

	// Create a test request with the JSON body
	jsoBody := []byte(`{
		"person_name_storage": "vendendor_testando",
		"storage_name": "storage_testando",
		"person_cpf": "12345678201",
		"city": "city_testando",
		"state": "state_testando"
	}`)

	req, err := http.NewRequest("POST", "/card_machine", bytes.NewBuffer(jsoBody)) // Create a new HTTP request with the JSON body
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json") // Set the Content-Type header to application/json

	recorder := httptest.NewRecorder() // Create a response recorder to capture the response

	router.ServeHTTP(recorder, req) //Server the reqquest using the router

	// Check the response status code
	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status code %d, but got %d", http.StatusOK, recorder.Code)
	}

	// Check the response body
	expectedResponse := `{"message":"Value inserted successfully"}`
	if recorder.Body.String() != expectedResponse {
		t.Errorf("Expected response body %s, but got %s", expectedResponse, recorder.Body.String())
	}
}

func TestHttPostSale(t *testing.T) {
	gin.SetMode(gin.TestMode) //Test mode for dont poluid my console with logs
	// Call the DbConnection function to test the database connection
	db, err, _ := repository.DbConnection()
	if err != nil {
		fmt.Println("Error in connection", err)
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
	paymentCase := usecases.NewPaymentCase(repo)
	kafkaCase := kafka_config.NewKafkaCaseDependency(produce)
	authCase := usecases.NewNewCardMachineCase(repo, paymentCase, kafkaCase)
	handlerCase := handler.NewHandlerCase(authCase)
	// Create the route
	router := gin.Default()
	router.POST("/postSales", handlerCase.PostSales)

	// Create a test request with the JSON body
	jsoBody := []byte(`{
		"amount": "1000"
	}`)

	req, err := http.NewRequest("POST", "/postSales", bytes.NewBuffer(jsoBody)) // Create a new HTTP request with the JSON body
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json") // Set the Content-Type header to application/json

	recorder := httptest.NewRecorder() // Create a response recorder to capture the response

	router.ServeHTTP(recorder, req) //Server the reqquest using the router

	// Check the response status code
	if recorder.Code != http.StatusCreated {
		t.Errorf("Expected status code %d, but got %d", http.StatusOK, recorder.Code)
	}

	// Check the response body
	expectedResponse := `{"message":"Value inserted successfully"}`
	if recorder.Body.String() != expectedResponse {
		t.Errorf("Expected response body %s, but got %s", expectedResponse, recorder.Body.String())
	}
}
