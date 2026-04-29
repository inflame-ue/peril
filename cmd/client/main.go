package main

import (
	"log"
	"strings"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
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

	queueName := strings.Join([]string{routing.PauseKey, username}, ".")
	pubsub.DeclareAndBind(amqpConnection, routing.ExchangePerilDirect, queueName, routing.PauseKey, pubsub.Transient)

	gameState := gamelogic.NewGameState(username)

outer:
	for {
		words := gamelogic.GetInput()
		command := words[0]

		switch command {
		case "spawn":
			err := gameState.CommandSpawn(words)
			if err != nil {
				log.Printf("failed to spawn unit: %v", err)
				continue
			}
		case "move":
			move, err := gameState.CommandMove(words)
			if err != nil {
				log.Printf("failed make a move: %v", err)
				continue
			}
			log.Printf("performed the move to location: %v", move.ToLocation)
		case "status":
			gameState.CommandStatus()
		case "help":
			gamelogic.PrintClientHelp()
		case "spam":
			log.Print("Spamming not allowed yet!")
		case "quit":
			gamelogic.PrintQuit()
			break outer
		default:
			log.Print("command not recognized...continuing...")
		}
	}
}
