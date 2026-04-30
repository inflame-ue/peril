package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

type AckType int

const (
	Ack AckType = iota
	NackRequeue
	NackDiscard
)

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
	data, err := json.Marshal(val)
	if err != nil {
		return fmt.Errorf("failed to marhal the value: %v", err)
	}

	err = ch.PublishWithContext(context.Background(), exchange, key, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        data,
	})
	if err != nil {
		return fmt.Errorf("failed to publish the message: %v", err)
	}
	return nil
}

func SubscribeJSON[T any](conn *amqp.Connection, exchange, queueName, key string, queueType SimpleQueueType, handler func(T) AckType) error {
	amqpChannel, amqpQueue, err := DeclareAndBind(conn, exchange, queueName, key, queueType)
	if err != nil {
		return err
	}

	deliveryChannel, err := amqpChannel.Consume(amqpQueue.Name, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("failed to consume the queue: %v", err)
	}

	go func() {
		for delivery := range deliveryChannel {
			var data T
			if err := json.Unmarshal(delivery.Body, &data); err != nil {
				log.Print("failed to umarhshal delivery message...skipping...")
				continue
			}

			ackType := handler(data)
			switch ackType {
			case Ack:
				log.Print("message acknowledge")
				delivery.Ack(false)
			case NackRequeue:
				log.Print("message negative acknowledge and requeue")
				delivery.Nack(false, true)
			case NackDiscard:
				log.Print("message negative acknowledge and discard to dead letter queue")
				delivery.Nack(false, false)
			}
		}
	}()

	return nil
}
