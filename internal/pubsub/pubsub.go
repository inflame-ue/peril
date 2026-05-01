package pubsub

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

type AckType int

const (
	Ack AckType = iota
	NackRequeue
	NackDiscard
)

func PublishGameLogSlug(amqpChannel *amqp.Channel, username, msg string) error {
	gl := routing.GameLog{
		CurrentTime: time.Now(),
		Message:     msg,
		Username:    username,
	}

	err := PublishGob(amqpChannel, routing.ExchangePerilTopic, routing.GameLogSlug+"."+username, gl)
	if err != nil {
		return err
	}

	return nil
}

func PublishGob[T any](ch *amqp.Channel, exchange, key string, val T) error {
	var buffer bytes.Buffer
	gobEnc := gob.NewEncoder(&buffer)

	if err := gobEnc.Encode(val); err != nil {
		return fmt.Errorf("failed to encode to gob: %v", err)
	}

	err := ch.PublishWithContext(context.Background(), exchange, key, false, false, amqp.Publishing{
		ContentType: "application/gob",
		Body:        buffer.Bytes(),
	})
	if err != nil {
		return fmt.Errorf("failed to publish the gob bytes: %v", err)
	}
	return nil
}

func SubscribeGob[T any](conn *amqp.Connection, exchange, queueName, key string, queueType SimpleQueueType, handler func(T) AckType) error {
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
			buffer := bytes.NewBuffer(delivery.Body)
			decoder := gob.NewDecoder(buffer)
			if err := decoder.Decode(&data); err != nil {
				log.Print("failed to decode from gob...skipping...")
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
