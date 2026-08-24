package worker

import (
	"context"
	"database/sql"
	"errors"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/service"
	"log/slog"
	"sync"
	"testing"
	"time"
)

type workerRepo struct {
	run         model.AutomationRun
	claim       bool
	finished    []model.RunState
	outbox      model.OutboxMessage
	outboxClaim bool
	delivered   []bool
}

func (w *workerRepo) ClaimRun(context.Context) (model.AutomationRun, error) {
	if w.claim {
		return model.AutomationRun{}, errors.New("none")
	}
	w.claim = true
	return w.run, nil
}
func (w *workerRepo) FinishRun(_ context.Context, _ int64, state model.RunState, _ string, _ time.Time) error {
	w.finished = append(w.finished, state)
	return nil
}
func (w *workerRepo) ClaimOutbox(context.Context) (model.OutboxMessage, error) {
	if w.outboxClaim {
		return model.OutboxMessage{}, errors.New("none")
	}
	w.outboxClaim = true
	return w.outbox, nil
}
func (w *workerRepo) MarkOutbox(_ context.Context, _ int64, ok bool, _ time.Time) error {
	w.delivered = append(w.delivered, ok)
	return nil
}

type workerAutomation struct {
	err    error
	called int
}

func (a *workerAutomation) Execute(context.Context, int64) error { a.called++; return a.err }

func TestBackoff(t *testing.T) {
	b := Backoff{Base: time.Second, Max: 8 * time.Second}
	cases := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 8 * time.Second}
	for i, want := range cases {
		if got := b.Duration(i + 1); got != want {
			t.Fatalf("attempt %d got %v want %v", i+1, got, want)
		}
	}
}
func TestSleepCancellation(t *testing.T) {
	done := make(chan struct{})
	close(done)
	if Sleep(done, time.Hour) {
		t.Fatal("cancelled sleep returned true")
	}
	if !Sleep(make(chan struct{}), time.Millisecond) {
		t.Fatal("completed sleep returned false")
	}
}
func TestMemoryPublisher(t *testing.T) {
	p := &MemoryPublisher{}
	msg := model.OutboxMessage{ID: 1, Topic: "device.updated"}
	if err := p.Publish(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	if len(p.Messages) != 1 {
		t.Fatal("message missing")
	}
	p.Err = errors.New("network")
	if err := p.Publish(context.Background(), msg); err == nil {
		t.Fatal("publisher error ignored")
	}
}
func TestSupervisorOnce(t *testing.T) {
	called := 0
	jobs := []Job{{Name: "one", Run: func(context.Context) error { called++; return nil }}, {Name: "two", Run: func(context.Context) error { called++; return nil }}}
	if err := RunOnce(context.Background(), jobs); err != nil {
		t.Fatal(err)
	}
	if called != 2 {
		t.Fatalf("called=%d", called)
	}
}
func TestSupervisorError(t *testing.T) {
	want := errors.New("failed")
	jobs := []Job{{Name: "bad", Run: func(context.Context) error { return want }}}
	if err := RunOnce(context.Background(), jobs); !errors.Is(err, want) {
		t.Fatalf("err=%v", err)
	}
}
func TestLifecycle(t *testing.T) {
	l := service.StartLifecycle(context.Background(), func(ctx context.Context) { <-ctx.Done() })
	if err := l.Stop(time.Second); err != nil {
		t.Fatal(err)
	}
	if err := l.Stop(time.Second); err != nil {
		t.Fatal(err)
	}
}
func TestWorkerDependencyContracts(t *testing.T) {
	var _ interface {
		ClaimRun(context.Context) (model.AutomationRun, error)
		FinishRun(context.Context, int64, model.RunState, string, time.Time) error
	} = (*workerRepo)(nil)
	var _ interface {
		Execute(context.Context, int64) error
	} = (*workerAutomation)(nil)
	_ = slog.Default()
}

// ackStore records acknowledgements and can block the AcknowledgeOutbox call
// until released, so a test can observe ordering relative to Publish and
// context cancellation.
type ackStore struct {
	mu         sync.Mutex
	ackd       []int64
	release    chan struct{}
	started    chan struct{}
	block      bool
	claimOnce  bool
	claimedMsg model.OutboxMessage
	failAck    error
}

func (s *ackStore) ClaimOutbox(context.Context) (model.OutboxMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.claimOnce {
		return model.OutboxMessage{}, sql.ErrNoRows
	}
	s.claimOnce = true
	return s.claimedMsg, nil
}
func (s *ackStore) AcknowledgeOutbox(_ context.Context, id int64) error {
	s.mu.Lock()
	if s.started != nil {
		select {
		case s.started <- struct{}{}:
		default:
		}
	}
	s.mu.Unlock()
	if s.block && s.release != nil {
		<-s.release
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failAck != nil {
		return s.failAck
	}
	s.ackd = append(s.ackd, id)
	return nil
}
func (s *ackStore) MarkOutboxFailed(context.Context, int64, string) error { return nil }
func (s *ackStore) RescheduleOutbox(context.Context, int64, int, time.Time, error) error {
	return nil
}

// blockingPublisher succeeds and signals when Publish returns, so the test can
// cancel the context in the window between webhook success and acknowledgement.
type blockingPublisher struct {
	published chan struct{}
}

func (p *blockingPublisher) Publish(context.Context, model.OutboxMessage) error {
	if p.published != nil {
		select {
		case p.published <- struct{}{}:
		default:
		}
	}
	return nil
}

// TestOutboxAcknowledgementSurvivesShutdown proves that an outbox message the
// external webhook already accepted is acknowledged before process returns,
// even when the worker context is cancelled immediately after Publish. This is
// the publish-vs-acknowledge shutdown window: previously the acknowledgement
// ran on a detached goroutine, so a restart redelivered the accepted message.
func TestOutboxAcknowledgementSurvivesShutdown(t *testing.T) {
	store := &ackStore{
		claimedMsg: model.OutboxMessage{ID: 7, Topic: "device.command", Attempts: 1},
		release:    make(chan struct{}),
		started:    make(chan struct{}, 1),
		block:      true,
	}
	published := make(chan struct{}, 1)
	runner := &OutboxRunner{
		Store:      store,
		Publisher:  &blockingPublisher{published: published},
		Logger:     slog.Default(),
		RetryLimit: 3,
		AckTimeout: time.Second,
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runner.process(ctx) }()
	<-published // webhook accepted the message
	// Cancel now, inside the publish-vs-acknowledge window, before AcknowledgeOutbox completes.
	cancel()
	<-store.started
	close(store.release) // let the bounded acknowledgement commit
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("process returned error after successful publish: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("process did not return; acknowledgement was not completed before shutdown")
	}
	store.mu.Lock()
	ackd := len(store.ackd)
	store.mu.Unlock()
	if ackd != 1 {
		t.Fatalf("expected delivered state persisted once, got %d", ackd)
	}
}

// TestOutboxAcknowledgementFailureRetainsMessage proves that when the durable
// acknowledgement write fails, the message is left in a redeliverable state
// (not silently dropped), so a later cycle can retry delivery.
func TestOutboxAcknowledgementFailureRetainsMessage(t *testing.T) {
	store := &ackStore{
		claimedMsg: model.OutboxMessage{ID: 9, Topic: "device.command", Attempts: 1},
		failAck:    errors.New("db unavailable"),
	}
	runner := &OutboxRunner{
		Store:      store,
		Publisher:  &blockingPublisher{},
		Logger:     slog.Default(),
		RetryLimit: 3,
		AckTimeout: time.Second,
	}
	if err := runner.process(context.Background()); err != nil {
		t.Fatalf("process returned error after successful publish: %v", err)
	}
	store.mu.Lock()
	ackd := len(store.ackd)
	store.mu.Unlock()
	if ackd != 0 {
		t.Fatalf("failed acknowledgement should not mark delivered, got %d", ackd)
	}
}

// TestOutboxWaitBlocksUntilRunReturns proves Wait tracks the run goroutine so
// shutdown callers can wait for an in-flight cycle within a bounded budget.
func TestOutboxWaitBlocksUntilRunReturns(t *testing.T) {
	store := &ackStore{
		claimedMsg: model.OutboxMessage{ID: 11, Topic: "device.command", Attempts: 1},
	}
	runner := &OutboxRunner{
		Store:        store,
		Publisher:    &MemoryPublisher{},
		Logger:       slog.Default(),
		RetryLimit:   3,
		PollInterval: time.Millisecond,
		AckTimeout:   time.Second,
	}
	ctx, cancel := context.WithCancel(context.Background())
	go runner.Run(ctx) //nolint:errcheck
	cancel()
	if err := runner.Wait(time.Second); err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
}
