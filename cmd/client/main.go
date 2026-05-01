package main

import (
	"log"
	"strconv"
	"strings"
	"time"

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
	gameState := gamelogic.NewGameState(username)
	if err != nil {
		log.Fatalf("failed to fetch the username: %v", err)
	}

	moveQueueChannel, err := amqpConnection.Channel()
	if err != nil {
		log.Fatalf("failed to create a move specific channel: %v", err)
	}
	moveQueueName := strings.Join([]string{"army_moves", username}, ".")
	err = pubsub.SubscribeJSON(amqpConnection, routing.ExchangePerilTopic, moveQueueName, "army_moves.*", pubsub.Transient, handlerMove(moveQueueChannel, gameState))
	if err != nil {
		log.Print(err)
	}	

	warQueueChannel, err := amqpConnection.Channel()
	if err != nil {
		log.Fatalf("failed to create a war specific channel: %v", err)
	}
	warMessagesRoutingKey := strings.Join([]string{routing.WarRecognitionsPrefix, "*"}, ".")
	err = pubsub.SubscribeJSON(amqpConnection, routing.ExchangePerilTopic, routing.WarRecognitionsPrefix, warMessagesRoutingKey, pubsub.Durable, handlerWarMessages(warQueueChannel, gameState))
	if err != nil {
		log.Print(err)
	}

	pauseQueueName := strings.Join([]string{routing.PauseKey, username}, ".")
	err = pubsub.SubscribeJSON(amqpConnection, routing.ExchangePerilDirect, pauseQueueName, routing.PauseKey, pubsub.Transient, handlerPause(gameState))
	if err != nil {
		log.Print(err)
	}

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
				log.Printf("failed to make a move: %v", err)
				continue
			}

			moveAMQPChannel, err := amqpConnection.Channel()
			if err != nil {
				log.Printf("failed to establish a move channel: %v", err)
				continue
			}

			err = pubsub.PublishJSON(moveAMQPChannel, routing.ExchangePerilTopic, routing.ArmyMovesPrefix+"."+move.Player.Username, move)
			if err != nil {
				log.Print(err)
				return
			}

			log.Printf("published the message to %v queue", moveQueueName)
			log.Printf("performed the move of %v to location: %v", move.Units, move.ToLocation)
		case "status":
			gameState.CommandStatus()
		case "help":
			gamelogic.PrintClientHelp()
		case "spam":
			if len(words) < 2 {
				log.Print("the number of spam messages must be provided")
				continue
			}
			spamNumber, err := strconv.Atoi(words[1])
			if err != nil {
				log.Print("failed to parse the spam number...skipping...")
				continue
			}
			for i := 0; i <= spamNumber; i++ {
				maliciousLog := gamelogic.GetMaliciousLog()
				gameLog := routing.GameLog{
					CurrentTime: time.Now(),
					Message: maliciousLog,
					Username: gameState.GetUsername(),
				}

				spamChannel, err := amqpConnection.Channel()
				if err != nil {
					log.Print("failed to establish a channel")
					continue
				}
				err = pubsub.PublishGob(spamChannel, routing.ExchangePerilTopic, routing.GameLogSlug + "." + gameState.GetUsername(), gameLog)
				if err != nil {
					log.Print("failed to publish the game log")
					continue
				}
			}
		case "quit":
			gamelogic.PrintQuit()
			break outer
		default:
			log.Print("command not recognized...continuing...")
		}
	}
}
