package worker

import (
	"context"
	"errors"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/repo"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/service"
	"log/slog"
	"sync"
	"time"
)

type Runner struct {
	Repo         *repo.Repository
	Automations  *service.AutomationService
	Logger       *slog.Logger
	Workers      int
	Retry        service.RetryPolicy
	PollInterval time.Duration
}

func New(r *repo.Repository, a *service.AutomationService, l *slog.Logger, n int) *Runner {
	if n < 1 {
		n = 1
	}
	return &Runner{Repo: r, Automations: a, Logger: l, Workers: n, Retry: service.RetryPolicy{Limit: 5, Base: 100 * time.Millisecond}, PollInterval: 100 * time.Millisecond}
}
func (w *Runner) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	for i := 0; i < w.Workers; i++ {
		wg.Add(1)
		go func(id int) { defer wg.Done(); w.loop(ctx, id) }(i)
	}
	wg.Wait()
	return ctx.Err()
}
func (w *Runner) loop(ctx context.Context, id int) {
	ticker := time.NewTicker(w.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.process(ctx, id)
		}
	}
}
func (w *Runner) process(ctx context.Context, workerID int) {
	run, err := w.Repo.ClaimRun(ctx)
	if err != nil {
		return
	}
	if err = w.Automations.Execute(ctx, run.ID); err != nil {
		if w.Logger != nil {
			w.Logger.Error("automation run failed", "worker", workerID, "run", run.ID, "error", err)
		}
		_ = w.Repo.FinishRun(context.WithoutCancel(ctx), run.ID, model.RunFailed, err.Error(), time.Now().UTC())
	}
}

type OutboxPublisher interface {
	Publish(context.Context, model.OutboxMessage) error
}
type OutboxRunner struct {
	Repo         *repo.Repository
	Publisher    OutboxPublisher
	Logger       *slog.Logger
	RetryLimit   int
	PollInterval time.Duration
}

func (w *OutboxRunner) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			w.process(ctx)
		}
	}
}
func (w *OutboxRunner) process(ctx context.Context) {
	msg, err := w.Repo.ClaimOutbox(ctx)
	if err != nil {
		return
	}
	pubErr := w.Publisher.Publish(ctx, msg)
	if pubErr == nil {
		_ = w.Repo.MarkOutbox(context.WithoutCancel(ctx), msg.ID, true, time.Time{})
		return
	}
	if msg.Attempts >= w.RetryLimit {
		_ = w.Repo.MarkOutbox(context.WithoutCancel(ctx), msg.ID, false, time.Now().UTC().Add(24*time.Hour))
		if w.Logger != nil {
			w.Logger.Error("outbox permanently failed", "id", msg.ID, "error", pubErr)
		}
		return
	}
	_ = w.Repo.MarkOutbox(context.WithoutCancel(ctx), msg.ID, false, time.Now().UTC().Add(time.Duration(msg.Attempts)*time.Second))
}

type MemoryPublisher struct {
	Err      error
	Mu       sync.Mutex
	Messages []model.OutboxMessage
}

func (p *MemoryPublisher) Publish(_ context.Context, m model.OutboxMessage) error {
	p.Mu.Lock()
	defer p.Mu.Unlock()
	if p.Err != nil {
		return p.Err
	}
	p.Messages = append(p.Messages, m)
	return nil
}

var ErrNoWork = errors.New("no work")
