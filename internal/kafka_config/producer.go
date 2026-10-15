package kafka_config

import (
	"fmt"
	"os"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

type NewKafkaCase struct {
	Producer *kafka.Producer
}

func NewKafkaCaseDependency(producer *kafka.Producer) *NewKafkaCase {
	return &NewKafkaCase{
		Producer: producer,
	}
}

func (k *NewKafkaCase) ProducerGo([]byte) {
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
	topic := "purchases"

	key := ""
	data := ""
	k.Producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Key:            []byte(key),
		Value:          []byte(data),
	}, nil)
}

func (k *NewKafkaCase) NewKafkaProducer() {
	produce, err := kafka.NewProducer(&kafka.ConfigMap{
		// User-specific properties that you must set
		"bootstrap.servers": "localhost:9092", //run in docker local
		// Fixed properties
		"acks": "all"})

	if err != nil {
		fmt.Printf("Failed to create producer: %s", err)
		os.Exit(1)
	}

	NewKafkaCaseDependency(produce)
}
