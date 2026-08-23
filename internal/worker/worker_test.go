package worker

import (
	"context"
	"errors"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/service"
	"log/slog"
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
