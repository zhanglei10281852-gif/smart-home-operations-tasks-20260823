package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
)

type blockingOutboxStore struct {
	ackStarted chan struct{}
	releaseAck chan struct{}
}

func (s *blockingOutboxStore) ClaimOutbox(context.Context) (model.OutboxMessage, error) {
	return model.OutboxMessage{ID: 91, HouseholdID: 4, Topic: "device.command"}, nil
}
func (s *blockingOutboxStore) AcknowledgeOutbox(context.Context, int64) error {
	close(s.ackStarted)
	<-s.releaseAck
	return nil
}
func (*blockingOutboxStore) MarkOutboxFailed(context.Context, int64, string) error {
	return errors.New("not used")
}
func (*blockingOutboxStore) RescheduleOutbox(context.Context, int64, int, time.Time, error) error {
	return errors.New("not used")
}

type successfulOutboxPublisher struct{ published chan struct{} }

func (p successfulOutboxPublisher) Publish(context.Context, model.OutboxMessage) error {
	close(p.published)
	return nil
}

func TestSuccessfulPublishWaitsForDurableAcknowledgement(t *testing.T) {
	store := &blockingOutboxStore{ackStarted: make(chan struct{}), releaseAck: make(chan struct{})}
	publisher := successfulOutboxPublisher{published: make(chan struct{})}
	runner := &OutboxRunner{Store: store, Publisher: publisher}
	done := make(chan error, 1)
	go func() { done <- runner.RunOnce(context.Background()) }()
	<-publisher.published
	<-store.ackStarted

	select {
	case err := <-done:
		close(store.releaseAck)
		if err != nil {
			t.Fatalf("successful outbox publish returned %v", err)
		}
		t.Fatal("worker returned before the published message was durably acknowledged")
	case <-time.After(60 * time.Millisecond):
	}
	close(store.releaseAck)
	if err := <-done; err != nil {
		t.Fatalf("outbox acknowledgement returned %v", err)
	}
}
