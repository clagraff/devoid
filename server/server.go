package server

import (
	"fmt"

	"github.com/clagraff/devoid/entities"
	"github.com/clagraff/devoid/intents"
	"github.com/clagraff/devoid/mutators"
	"github.com/clagraff/devoid/network"
	"github.com/clagraff/devoid/pubsub"

	uuid "github.com/satori/go.uuid"
)

func Serve(locker *entities.Locker, tunnels chan network.Tunnel) {
	intentsQueue := make(chan intents.Intent, 100)
	notificationsQueue := make(chan pubsub.Notification, 100)
	messagesQueue := make(chan network.Message, 100)
	subscriberQueue := make(chan pubsub.Subscriber, 100)

	go handleTunnels(locker, tunnels, messagesQueue, intentsQueue, subscriberQueue)
	go handleIntents(locker, intentsQueue, notificationsQueue)
	go handleNotifications(
		notificationsQueue,
		messagesQueue,
		subscriberQueue,
	)

	select {}
}

func handleTunnels(
	locker *entities.Locker,
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
			handleSubscribe(locker, tunnel, subscriberQueue)
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
	locker *entities.Locker,
	tunnel network.Tunnel,
	subscriberQueue chan pubsub.Subscriber,
) {
	container, err := locker.GetByID(tunnel.ID)
	if err != nil {
		panic(err)
	}
	container.RLock()
	defer container.RUnlock()

	entity := container.GetEntity()

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
			nil),
	}

	for _, sub := range subscribers {
		subscriberQueue <- sub
	}
}

func handleNotifications(
	queue chan pubsub.Notification,
	messagesQueue chan network.Message,
	subscriberQueue chan pubsub.Subscriber,
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
				delete(subscribers, notification.Type)
			}
		}
	}
}

func handleIntents(
	locker *entities.Locker,
	queue chan intents.Intent,
	notificationQueue chan pubsub.Notification,
) {
	for intent := range queue {
		serverMutations, notifications := handleIntent(locker, intent)
		for _, mutation := range serverMutations {
			mutation.Mutate(locker)
		}

		for _, notification := range notifications {
			notificationQueue <- notification
		}
	}
}

func handleIntent(locker *entities.Locker, intent intents.Intent) ([]mutators.Mutator, []pubsub.Notification) {
	return intent.Compute(locker)
}
