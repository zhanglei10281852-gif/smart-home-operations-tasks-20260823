package worker

import (
	"context"
	"database/sql"
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
	PollInterval time.Duration
}

func New(r *repo.Repository, a *service.AutomationService, l *slog.Logger, n int) *Runner {
	if n < 1 {
		n = 1
	}
	return &Runner{Repo: r, Automations: a, Logger: l, Workers: n, PollInterval: 100 * time.Millisecond}
}
func (w *Runner) Run(ctx context.Context) error {
	if w == nil || w.Repo == nil || w.Automations == nil {
		return errors.New("automation runner is not configured")
	}
	if w.Workers < 1 {
		w.Workers = 1
	}
	if w.PollInterval <= 0 {
		w.PollInterval = 100 * time.Millisecond
	}
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
		if !errors.Is(err, sql.ErrNoRows) && w.Logger != nil {
			w.Logger.Error("automation claim failed", "worker", workerID, "error", err)
		}
		return
	}
	if err = w.Automations.Execute(ctx, run.ID); err != nil {
		if w.Logger != nil {
			w.Logger.Error("automation run failed", "worker", workerID, "run", run.ID, "error", err)
		}
		persistCtx, cancel := persistenceContext(ctx)
		defer cancel()
		if finishErr := HandleRunFailure(persistCtx, w.Repo, run.ID, err, time.Now().UTC()); finishErr != nil && w.Logger != nil {
			w.Logger.Error("automation failure state was not persisted", "worker", workerID, "run", run.ID, "error", finishErr)
		}
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
	if w == nil || w.Repo == nil || w.Publisher == nil {
		return errors.New("outbox runner is not configured")
	}
	if w.PollInterval <= 0 {
		w.PollInterval = 100 * time.Millisecond
	}
	if w.RetryLimit < 1 {
		w.RetryLimit = 5
	}
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
		if !errors.Is(err, sql.ErrNoRows) && w.Logger != nil {
			w.Logger.Error("outbox claim failed", "error", err)
		}
		return
	}
	pubErr := w.Publisher.Publish(ctx, msg)
	persistCtx, cancel := persistenceContext(ctx)
	defer cancel()
	if pubErr == nil {
		if err := w.Repo.MarkOutbox(persistCtx, msg.ID, true, time.Time{}); err != nil && w.Logger != nil {
			w.Logger.Error("outbox delivery state was not persisted", "id", msg.ID, "error", err)
		}
		return
	}
	if msg.Attempts >= w.RetryLimit {
		if err := w.Repo.MarkOutboxFailed(persistCtx, msg.ID, pubErr.Error()); err != nil && w.Logger != nil {
			w.Logger.Error("outbox permanent failure was not persisted", "id", msg.ID, "error", err)
		}
		if w.Logger != nil {
			w.Logger.Error("outbox permanently failed", "id", msg.ID, "error", pubErr)
		}
		return
	}
	next := time.Now().UTC().Add(time.Duration(msg.Attempts) * time.Second)
	if err := w.Repo.RescheduleOutbox(persistCtx, msg.ID, msg.Attempts, next, pubErr); err != nil && w.Logger != nil {
		w.Logger.Error("outbox retry was not persisted", "id", msg.ID, "error", err)
	}
}

func persistenceContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(parent), 5*time.Second)
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
