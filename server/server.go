package server

import (
	"fmt"

	"github.com/clagraff/devoid/intents"
	"github.com/clagraff/devoid/mutators"
	"github.com/clagraff/devoid/network"
	"github.com/clagraff/devoid/pubsub"
	"github.com/clagraff/devoid/state"

	uuid "github.com/satori/go.uuid"
)

func Serve(state *state.State, tunnels chan network.Tunnel) {
	intentsQueue := make(chan intents.Intent, 100)
	notificationsQueue := make(chan pubsub.Notification, 100)
	messagesQueue := make(chan network.Message, 100)
	mutatorsQueue := make(chan mutators.Mutator, 100)
	subscriberQueue := make(chan pubsub.Subscriber, 100)

	go handleMutators(state, mutatorsQueue)
	go handleTunnels(state, tunnels, messagesQueue, intentsQueue, subscriberQueue)
	go handleIntents(state, intentsQueue, notificationsQueue)
	go handleNotifications(
		notificationsQueue,
		messagesQueue,
		subscriberQueue,
		mutatorsQueue,
	)

	notify := func(notification pubsub.Notification) bool {
		for _, mutator := range notification.Mutators {
			mutatorsQueue <- mutator
		}

		return true
	}

	subscriberQueue <- pubsub.MakeSubscriber(
		notify,
		nil,
	)

	select {}
}

func handleMutators(state *state.State, queue chan mutators.Mutator) {
	for mutator := range queue {
		handleMutator(state, mutator)
	}
}

func handleTunnels(
	state *state.State,
	tunnels chan network.Tunnel,
	messagesQueue chan network.Message,
	intentsQueue chan intents.Intent,
	subscriberQueue chan pubsub.Subscriber,
) {
	availableTunnels := make(map[uuid.UUID]network.Tunnel)
	for {
		select {
		case tunnel := <-tunnels:
			availableTunnels[tunnel.ID] = tunnel
			handleSubscribe(state, tunnel, subscriberQueue)
			intentsQueue <- intents.Perceive{SourceID: tunnel.ID}
		case message := <-messagesQueue:
			clientID := message.ClientID
			if tunnel, ok := availableTunnels[clientID]; ok {
				tunnel.Outgoing <- message
			} else {
				fmt.Println("tunnel no longer available for client to receive outgoing message")
			}
		default:
			// no-op
		}

		for _, tunnel := range availableTunnels {
			select {
			case _ = <-tunnel.Closed:
				delete(availableTunnels, tunnel.ID)
			case message := <-tunnel.Incoming:
				intent, err := intents.Unmarshal(message.ContentType, message.Content)
				if err != nil {
					tunnel.Closed <- struct{}{}
					panic(err)
				}

				intentsQueue <- intent
			default:
				// no-op
			}
		}
	}
}

func handleSubscribe(
	state *state.State,
	tunnel network.Tunnel,
	subscriberQueue chan pubsub.Subscriber,
) {
	entity, unlock, ok := state.ByID(tunnel.ID)
	if !ok {
		panic("could not locate ID by tunnel id")
	}
	defer unlock()

	subscribers := []pubsub.Subscriber{
		pubsub.MakeSubscriber(
			func(notification pubsub.Notification) bool {
				for _, mutator := range notification.Mutators {
					tunnel.Outgoing <- network.MakeMessage(
						tunnel.ID,
						mutator,
					)
				}
				return true
			},
			entity.Position,
			entity.ID,
		),
	}

	for _, sub := range subscribers {
		subscriberQueue <- sub
	}
}

func handleNotifications(
	queue chan pubsub.Notification,
	messagesQueue chan network.Message,
	subscriberQueue chan pubsub.Subscriber,
	mutatorsQueue chan mutators.Mutator,
) {
	subscribers := make(map[interface{}][]pubsub.Subscriber)

	for {
		select {
		case notification := <-queue:
			handleNotification(notification, subscribers)
		case sub := <-subscriberQueue:
			for _, noticeType := range sub.NotifyOn() {
				if _, ok := subscribers[noticeType]; !ok {
					subscribers[noticeType] = make([]pubsub.Subscriber, 0)
				}
				subscribers[noticeType] = append(subscribers[noticeType], sub)
			}
		default:
			// no-op
		}
	}
}

func handleNotification(
	notification pubsub.Notification,
	subscribers map[interface{}][]pubsub.Subscriber,
) {
	var currentSubs []pubsub.Subscriber
	var ok bool

	currentSubs, ok = subscribers[notification.Type]

	if !ok {
		fmt.Printf("no subscribers to %T%+v\n", notification.Type, notification.Type)
		return

	}

	for i := len(currentSubs) - 1; i >= 0; i-- {
		sub := currentSubs[i]
		retain := sub.Notify(notification)

		if !retain {
			currentSubs = append(
				currentSubs[:i],
				currentSubs[i+1:]...,
			)

			if len(currentSubs) == 0 {
				fmt.Printf("no more subscribers for %T\n", notification.Type)
				delete(subscribers, notification.Type)
			}
		}
	}
}

func handleMutator(state *state.State, mutator mutators.Mutator) {
	fmt.Printf("handling mutator %T\n", mutator)
	mutator.Mutate(state)
}

func handleIntents(
	state *state.State,
	queue chan intents.Intent,
	notificationQueue chan pubsub.Notification,
) {
	for intent := range queue {
		notifications := handleIntent(state, intent)
		for _, notification := range notifications {
			fmt.Printf(
				"sending notification on %T for intent %T\n",
				notification.Type,
				intent,
			)
			notificationQueue <- notification
		}
	}
}

func handleIntent(state *state.State, intent intents.Intent) []pubsub.Notification {
	fmt.Printf("handling intent %T\n", intent)
	return intent.Compute(state)
}
