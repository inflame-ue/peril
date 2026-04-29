package main

import (
	"log"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
)

func main() {
	connectionString := "amqp://guest:guest@localhost:5672/"
	amqpConnection, err := amqp.Dial(connectionString)
	if err != nil {
		log.Fatalf("failed to connect to rabbitmq: %v", err)
	}

	username, err := gamelogic.ClientWelcome()
	if err != nil {
		log.Fatalf("failed to fetch the username: %v", err)
	}

	
}
