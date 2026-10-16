package kafka_config

import (
	"card_machine/internal/model"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

type NewConsumerCase struct {
	Consumer *kafka.Consumer
}

func NewConsumerDependency(consumer *kafka.Consumer) *NewConsumerCase {
	return &NewConsumerCase{
		Consumer: consumer,
	}
}

func (c *NewConsumerCase) ConsumerGo() error {
	topic := "sale"
	err := c.Consumer.SubscribeTopics([]string{topic}, nil)
	if err != nil {
		return err
	}
	//set up a channel for handling Ctrl-c
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

	//Process messages
	run := true
	for run {
		select {
		case sig := <-sigchan:
			fmt.Printf("Caught signal %v: termianting\n", sig)
			run = false //exit the for loop

		default:
			ev, err := c.Consumer.ReadMessage(100 * time.Millisecond) //time for created a time out\
			if err != nil {
				continue
			}
			var event model.PaymentKafka
			err = json.Unmarshal(ev.Value, &event)
			if err != nil {
				continue
			}
			fmt.Println(event.SaleId)
			fmt.Println(event.Value)
			fmt.Println(event.Status)
			fmt.Printf("Consumed event from topic %s: key = %-10s value = %s\n",
				*ev.TopicPartition.Topic, string(ev.Key), string(ev.Value))
		}

	}
	defer c.Consumer.Close()
	return nil
}

func NewKafkaConsumer() (*kafka.Consumer, error) {
	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		// User-specific properties that you must set
		"bootstrap.servers": "localhost:9092",
		"group.id":          "card-machine",
		"auto.offset.reset": "earliest"})

	if err != nil {
		fmt.Printf("Failed to create consumer: %s", err)
		os.Exit(1)
	}

	return c, nil
}
