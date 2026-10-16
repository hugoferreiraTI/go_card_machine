package kafka_config

import (
	"card_machine/internal/model"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

type NewProducerCase struct {
	Producer *kafka.Producer
}

func NewProducerCaseDependency(producer *kafka.Producer) *NewProducerCase {
	return &NewProducerCase{
		Producer: producer,
	}
}

func (k *NewProducerCase) ProducerGo(event model.PaymentKafka) error {
	//goroutine to handle message delivery reports and possibbly other event types
	go func() {
		for e := range k.Producer.Events() {
			switch ev := e.(type) {
			case *kafka.Message:
				if ev.TopicPartition.Error != nil {
					fmt.Printf("Faield to deliver message: %v\n", ev.TopicPartition)
				} else {
					fmt.Printf("Produce event to topic %s: key = %-10s value = %s\n", *ev.TopicPartition.Topic, string(ev.Key), string(ev.Value))
				}
			}
		}
	}()
	topic := "sale"

	key := strconv.Itoa(event.SaleId)

	data, err := json.Marshal(event) //converte the JSON in byte;
	if err != nil {
		return err
	}

	k.Producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Key:            []byte(key),
		Value:          []byte(data),
	}, nil)

	return nil
}

func NewKafkaProducer() (*kafka.Producer, error) {
	produce, err := kafka.NewProducer(&kafka.ConfigMap{
		// User-specific properties that you must set
		"bootstrap.servers": "localhost:9092", //run in docker local
		// Fixed properties
		"acks": "all"})

	if err != nil {
		fmt.Printf("Failed to create producer: %s", err)
		os.Exit(1)
	}
	NewProducerCaseDependency(produce)
	return produce, nil
}
