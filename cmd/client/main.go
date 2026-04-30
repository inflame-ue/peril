package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func handlerPause(gameState *gamelogic.GameState) func(routing.PlayingState) pubsub.AckType {
	return func(playingState routing.PlayingState) pubsub.AckType {
		defer fmt.Print("> ")
		gameState.HandlePause(playingState)
		return pubsub.Ack
	}
}

func handlerMove(amqpChannel *amqp.Channel, gameState *gamelogic.GameState) func(gamelogic.ArmyMove) pubsub.AckType {
	return func(move gamelogic.ArmyMove) pubsub.AckType {
		defer fmt.Print("> ")
		moveOutcome := gameState.HandleMove(move)

		if moveOutcome == gamelogic.MoveOutcomeMakeWar {
			routingKey := strings.Join([]string{routing.WarRecognitionsPrefix, move.Player.Username}, ".")
			dataToPublish := gamelogic.RecognitionOfWar{
				Attacker: move.Player,
				Defender: gameState.GetPlayerSnap(),
			}
			pubsub.PublishJSON(amqpChannel, routing.ExchangePerilTopic, routingKey, dataToPublish)
			return pubsub.NackRequeue
		}

		if moveOutcome == gamelogic.MoveOutComeSafe {
			return pubsub.Ack
		}

		return pubsub.NackDiscard
	}
}

func handlerWarMessages(gameState *gamelogic.GameState) func(gamelogic.RecognitionOfWar) pubsub.AckType {
	return func(msg gamelogic.RecognitionOfWar) pubsub.AckType {
		defer fmt.Print("> ")
		outcome, _, _ := gameState.HandleWar(msg)

		switch outcome {
		case gamelogic.WarOutcomeNotInvolved:
			return pubsub.NackRequeue
		case gamelogic.WarOutcomeNoUnits:
			return pubsub.NackDiscard
		case gamelogic.WarOutcomeOpponentWon:
			return pubsub.Ack
		case gamelogic.WarOutcomeYouWon:
			return pubsub.Ack
		case gamelogic.WarOutcomeDraw:
			return pubsub.Ack
		default:
			log.Print("uknown war outcome...discarding message...")
			return pubsub.NackDiscard
		}
	}
}

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

	pauseQueueName := strings.Join([]string{routing.PauseKey, username}, ".")
	gameState := gamelogic.NewGameState(username)
	pubsub.SubscribeJSON(amqpConnection, routing.ExchangePerilDirect, pauseQueueName, routing.PauseKey, pubsub.Transient, handlerPause(gameState))

	moveQueueChannel, err := amqpConnection.Channel()
	if err != nil {
		log.Fatalf("failed to create a move specific channel: %v", err)
	}
	moveQueueName := strings.Join([]string{"army_moves", username}, ".")
	pubsub.SubscribeJSON(amqpConnection, routing.ExchangePerilTopic, moveQueueName, "army_moves.*", pubsub.Transient, handlerMove(moveQueueChannel, gameState))

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

			err = pubsub.PublishJSON(moveAMQPChannel, routing.ExchangePerilTopic, moveQueueName, move)
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
			log.Print("Spamming not allowed yet!")
		case "quit":
			gamelogic.PrintQuit()
			break outer
		default:
			log.Print("command not recognized...continuing...")
		}
	}
}
