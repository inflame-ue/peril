package pubsub

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type SimpleQueueType int
const (
	Durable SimpleQueueType = iota
	Transient
)

func DeclareAndBind(conn *amqp.Connection, exchange, queueName, key string, queueType SimpleQueueType) (*amqp.Channel, amqp.Queue, error) {
	amqpChannel, err := conn.Channel()
	if err != nil {
		return &amqp.Channel{}, amqp.Queue{}, fmt.Errorf("failed to create channel for queue: %v", err)
	}

	var queueDurable bool
	if queueType == Durable {
		queueDurable = true
	}

	var queueAutoDelete, queueExclusive bool
	if queueType == Transient {
		queueAutoDelete = true
		queueExclusive = true
	}

	amqpQueue, err := amqpChannel.QueueDeclare(
		queueName,
		queueDurable, 
		queueAutoDelete,
		queueExclusive,
		false,
		nil,
	)
	if err != nil {
		return &amqp.Channel{}, amqp.Queue{}, fmt.Errorf("failed to declare the queue: %v", err)
	}

	err = amqpChannel.QueueBind(amqpQueue.Name, key, exchange, false, nil)
	if err != nil {
		return &amqp.Channel{}, amqp.Queue{}, fmt.Errorf("failed to bind the queue: %v", err)
	}

	return amqpChannel, amqpQueue, nil
}