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

		switch moveOutcome {
		case gamelogic.MoveOutcomeSamePlayer:
			return pubsub.Ack
		case gamelogic.MoveOutComeSafe:
			return pubsub.Ack
		case gamelogic.MoveOutcomeMakeWar:
			err := pubsub.PublishJSON(
				amqpChannel,
				routing.ExchangePerilTopic,
				routing.WarRecognitionsPrefix+"."+gameState.GetUsername(),
				gamelogic.RecognitionOfWar{
					Attacker: move.Player,
					Defender: gameState.GetPlayerSnap(),
				},
			)
			if err != nil {
				log.Printf("faild to publish json: %v", err)
				return pubsub.NackRequeue
			}
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
		}

		return pubsub.NackDiscard
	}
}

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

	warMessagesRoutingKey := strings.Join([]string{routing.WarRecognitionsPrefix, "*"}, ".")
	err = pubsub.SubscribeJSON(amqpConnection, routing.ExchangePerilTopic, routing.WarRecognitionsPrefix, warMessagesRoutingKey, pubsub.Durable, handlerWarMessages(gameState))
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
			log.Print("Spamming not allowed yet!")
		case "quit":
			gamelogic.PrintQuit()
			break outer
		default:
			log.Print("command not recognized...continuing...")
		}
	}
}
