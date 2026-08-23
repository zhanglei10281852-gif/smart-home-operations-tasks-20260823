package domain

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type hedgedNotifier struct {
	started chan struct{}
	release chan struct{}
	calls   atomic.Int64
}

func (n *hedgedNotifier) Send(context.Context, Notification) error {
	n.calls.Add(1)
	n.started <- struct{}{}
	<-n.release
	return nil
}

func TestAlertDeliveryDoesNotDuplicateASlowProvider(t *testing.T) {
	notifier := &hedgedNotifier{started: make(chan struct{}, 2), release: make(chan struct{})}
	msg := Notification{Recipient: "owner@example.com", Channel: "push", Subject: "alert", Body: "critical"}
	finished := make(chan error, 1)
	go func() { finished <- SendHedged(context.Background(), notifier, msg, 10*time.Millisecond) }()
	select {
	case <-notifier.started:
	case <-time.After(time.Second):
		t.Fatal("first provider call did not start")
	}
	select {
	case <-notifier.started:
	case <-time.After(time.Second):
		close(notifier.release)
		t.Fatal("hedged provider call did not start")
	}
	close(notifier.release)
	if err := <-finished; err != nil {
		t.Fatalf("send failed: %v", err)
	}
	if got := notifier.calls.Load(); got != 1 {
		t.Fatalf("notification was delivered %d times", got)
	}
}
