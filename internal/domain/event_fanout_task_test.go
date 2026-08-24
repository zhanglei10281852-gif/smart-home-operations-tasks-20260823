package domain

import (
	"errors"
	"testing"
	"time"
)

type controlledEventConsumer func(Event) error

func (consume controlledEventConsumer) Consume(event Event) error { return consume(event) }

func TestEventPublishWaitsForEveryStartedConsumer(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	completed := make(chan struct{})

	slow := controlledEventConsumer(func(Event) error {
		close(started)
		<-release
		close(completed)
		return nil
	})
	rejected := controlledEventConsumer(func(Event) error {
		<-started
		return errors.New("audit subscriber unavailable")
	})
	event := Event{
		Topic:       "device.command.accepted",
		HouseholdID: 7,
		ObjectType:  "device",
		ObjectID:    "42",
		Action:      "accepted",
		RequestID:   "req-fanout-ownership",
		At:          time.Now().UTC(),
	}

	returned := make(chan error, 1)
	go func() { returned <- (Fanout{Consumers: []EventConsumer{slow, rejected}}).Publish(event) }()

	select {
	case err := <-returned:
		close(release)
		<-completed
		if err == nil {
			t.Fatal("event rejection was lost")
		}
		t.Fatal("Publish returned while a started consumer still owned the event")
	case <-time.After(150 * time.Millisecond):
		close(release)
	}

	if err := <-returned; err == nil {
		t.Fatal("event rejection was lost after all consumers completed")
	}
}
