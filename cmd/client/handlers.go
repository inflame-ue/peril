package main

import (
	"fmt"
	"log"

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

func handlerWarMessages(amqpChannel *amqp.Channel, gameState *gamelogic.GameState) func(gamelogic.RecognitionOfWar) pubsub.AckType {
	return func(msg gamelogic.RecognitionOfWar) pubsub.AckType {
		defer fmt.Print("> ")
		outcome, winner, loser := gameState.HandleWar(msg)
		var outcomeMsg string

		switch outcome {
		case gamelogic.WarOutcomeNotInvolved:
			return pubsub.NackRequeue
		case gamelogic.WarOutcomeNoUnits:
			return pubsub.NackDiscard
		case gamelogic.WarOutcomeOpponentWon:
			outcomeMsg = winner + " won a war against " + loser
		case gamelogic.WarOutcomeYouWon:
			outcomeMsg = winner + " won a war against " + loser
		case gamelogic.WarOutcomeDraw:
			outcomeMsg = "A war between " + winner + " and " + loser + " resulted in a draw"
		}

		err := pubsub.PublishGameLogSlug(amqpChannel, gameState.GetUsername(), outcomeMsg)
		if err != nil {
			log.Printf("failed to publish game log slug: %v", err)
			return pubsub.NackRequeue
		}

		return pubsub.Ack
	}
}
