package main

import (
	"log"
	"os"
	"os/signal"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	// this will display the commands the user of the REPL can use
	gamelogic.PrintServerHelp()
	connectionString := "amqp://guest:guest@localhost:5672/"

	amqpConnection, err := amqp.Dial(connectionString)
	if err != nil {
		log.Fatalf("failed to connect to rabbitmq: %v", err)
	}
	defer amqpConnection.Close()

	log.Print("connection established with no problem")

	amqpChannel, err := amqpConnection.Channel()
	if err != nil {
		log.Fatalf("failed to create a channel over the connection: %v", err)
	}

	for {
		words := gamelogic.GetInput()
		if len(words) == 0 {
			continue
		}

		firstWord := words[0]
		if firstWord == "pause" {
			log.Print("sending a pause message to the exchange...")

			pubsub.PublishJSON(amqpChannel, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{
				IsPaused: true,
			})

			log.Print("pause message published with no problem")
		} else if firstWord == "resume" {
			log.Print("sending a resume message to the exchange...")

			pubsub.PublishJSON(amqpChannel, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{
				IsPaused: false,
			})

			log.Print("resume message published with no problem")
		} else if firstWord == "quit" {
			log.Print("exiting the application...")
			break
		} else {
			log.Print("unknown command")
		}
	}
	log.Print("shutting down...")
}
