package server

import (
	"fmt"

	"github.com/clagraff/devoid/actions"
	"github.com/clagraff/devoid/commands"
	"github.com/clagraff/devoid/entities"
	"github.com/clagraff/devoid/network"
	"github.com/clagraff/devoid/pubsub"

	uuid "github.com/satori/go.uuid"
)

func Serve(locker *entities.Locker, tunnels chan network.Tunnel) {
	commandsQueue := make(chan commands.Command, 100)
	notificationsQueue := make(chan pubsub.Notification, 100)
	messagesQueue := make(chan network.Message, 100)
	subscriberQueue := make(chan pubsub.Subscriber, 100)

	go handleTunnels(locker, tunnels, messagesQueue, commandsQueue, subscriberQueue)
	go handleCommands(locker, commandsQueue, notificationsQueue)
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
	commandsQueue chan commands.Command,
	subscriberQueue chan pubsub.Subscriber,
) {
	availableTunnels := make(map[uuid.UUID]network.Tunnel)
	for {
		select {
		case tunnel := <-tunnels:
			availableTunnels[tunnel.ID] = tunnel
			handleSubscribe(locker, tunnel, subscriberQueue)
			commandsQueue <- commands.Perceive{SourceID: tunnel.ID}
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
				command, err := commands.Unmarshal(message.ContentType, message.Content)
				if err != nil {
					tunnel.Closed <- struct{}{}
					panic(err)
				}

				commandsQueue <- command
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
	entity, err := locker.GetByID(tunnel.ID)
	if err != nil {
		panic(err)
	}

	subscribers := []pubsub.Subscriber{
		pubsub.MakeSubscriber(
			func(notification pubsub.Notification) bool {
				for _, action := range notification.Actions {
					tunnel.Outgoing <- network.MakeMessage(
						tunnel.ID,
						action,
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

func handleCommands(
	locker *entities.Locker,
	queue chan commands.Command,
	notificationQueue chan pubsub.Notification,
) {
	for command := range queue {
		serverMutations, notifications := handleCommand(locker, command)
		for _, mutation := range serverMutations {
			mutation.Execute(locker)
		}

		for _, notification := range notifications {
			notificationQueue <- notification
		}
	}
}

func handleCommand(locker *entities.Locker, command commands.Command) ([]actions.Action, []pubsub.Notification) {
	return command.Compute(locker)
}
