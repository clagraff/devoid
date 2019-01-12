package pubsub

import (
	"github.com/clagraff/devoid/mutators"
)

type Notification struct {
	Type     interface{}
	Mutators []mutators.Mutator
}

type Subscriber interface {
	Notify(Notification) bool
	NotifyOn() []interface{}
}

type customSubscriber struct {
	notify   func(Notification) bool
	notifyOn []interface{}
}

func (sub customSubscriber) Notify(notification Notification) bool {
	return sub.notify(notification)
}

func (sub customSubscriber) NotifyOn() []interface{} {
	return sub.notifyOn
}

func MakeSubscriber(notify func(Notification) bool, notifyOn ...interface{}) Subscriber {
	return customSubscriber{
		notify:   notify,
		notifyOn: notifyOn,
	}
}
