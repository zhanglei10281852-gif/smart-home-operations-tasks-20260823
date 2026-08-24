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
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			if requeueErr := w.Repo.RequeueRun(persistCtx, run.ID); requeueErr != nil && w.Logger != nil {
				w.Logger.Error("cancelled automation run was not requeued", "worker", workerID, "run", run.ID, "error", requeueErr)
			}
			return
		}
		if finishErr := w.Repo.FinishRun(persistCtx, run.ID, model.RunFailed, err.Error(), time.Now().UTC()); finishErr != nil && w.Logger != nil {
			w.Logger.Error("automation failure state was not persisted", "worker", workerID, "run", run.ID, "error", finishErr)
		}
	}
}

type OutboxPublisher interface {
	Publish(context.Context, model.OutboxMessage) error
}
type OutboxStore interface {
	ClaimOutbox(context.Context) (model.OutboxMessage, error)
	AcknowledgeOutbox(context.Context, int64) error
	MarkOutboxFailed(context.Context, int64, string) error
	RescheduleOutbox(context.Context, int64, int, time.Time, error) error
}
type OutboxRunner struct {
	Repo         *repo.Repository
	Store        OutboxStore
	Publisher    OutboxPublisher
	Logger       *slog.Logger
	RetryLimit   int
	PollInterval time.Duration
	AckTimeout   time.Duration

	doneMu sync.Mutex
	done   chan struct{}
}

func (w *OutboxRunner) outboxStore() OutboxStore {
	if w.Store != nil {
		return w.Store
	}
	return w.Repo
}

func (w *OutboxRunner) ackTimeout() time.Duration {
	if w.AckTimeout > 0 {
		return w.AckTimeout
	}
	return 5 * time.Second
}

// prepare initializes the shutdown signal channel on the caller's goroutine so
// that Wait can observe it without racing the Run goroutine. It is idempotent
// and safe to call from either the Run or Wait side.
func (w *OutboxRunner) prepare() chan struct{} {
	w.doneMu.Lock()
	defer w.doneMu.Unlock()
	if w.done == nil {
		w.done = make(chan struct{})
	}
	return w.done
}

func (w *OutboxRunner) Run(ctx context.Context) error {
	if w == nil || w.outboxStore() == nil || w.Publisher == nil {
		return errors.New("outbox runner is not configured")
	}
	if w.PollInterval <= 0 {
		w.PollInterval = 100 * time.Millisecond
	}
	if w.RetryLimit < 1 {
		w.RetryLimit = 5
	}
	done := w.prepare()
	defer close(done)
	ticker := time.NewTicker(w.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			// The loop is exiting, but process may still be acknowledging a
			// message it just published. Because acknowledgement is synchronous
			// and uses a cancellation-independent context, the delivered state
			// has already been durably recorded before process returned, so no
			// externally received message is left un-acknowledged.
			return ctx.Err()
		case <-ticker.C:
			w.process(ctx)
		}
	}
}

// Wait blocks until Run has fully returned, or until timeout elapses. Callers
// use it during shutdown to give an in-flight publish+acknowledge cycle a
// bounded opportunity to finish before the process exits, avoiding redelivery
// of a message the external webhook already accepted.
func (w *OutboxRunner) Wait(timeout time.Duration) error {
	done := w.prepare()
	if timeout <= 0 {
		<-done
		return nil
	}
	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return ErrDrainTimeout
	}
}
func (w *OutboxRunner) RunOnce(ctx context.Context) error {
	if w == nil || w.outboxStore() == nil || w.Publisher == nil {
		return errors.New("outbox runner is not configured")
	}
	return w.process(ctx)
}
func (w *OutboxRunner) process(ctx context.Context) error {
	store := w.outboxStore()
	msg, err := store.ClaimOutbox(ctx)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) && w.Logger != nil {
			w.Logger.Error("outbox claim failed", "error", err)
		}
		return err
	}
	// Publish and acknowledge within the same critical section so the delivered
	// state is durably recorded before process returns. Acknowledgement runs on
	// a cancellation-independent, bounded context: if shutdown cancels ctx after
	// the webhook already returned success, the AcknowledgeOutbox write still
	// commits before process yields. This closes the window where a message
	// accepted by the external system could be redelivered after a restart.
	pubErr := w.Publisher.Publish(ctx, msg)
	if pubErr == nil {
		if ackErr := w.acknowledgePublished(ctx, store, msg.ID); ackErr != nil && w.Logger != nil {
			w.Logger.Error("outbox delivery state was not persisted", "id", msg.ID, "error", ackErr)
		}
		return nil
	}
	persistCtx, cancel := persistenceContext(ctx)
	defer cancel()
	if msg.Attempts >= w.RetryLimit {
		if err := store.MarkOutboxFailed(persistCtx, msg.ID, pubErr.Error()); err != nil && w.Logger != nil {
			w.Logger.Error("outbox permanent failure was not persisted", "id", msg.ID, "error", err)
		}
		if w.Logger != nil {
			w.Logger.Error("outbox permanently failed", "id", msg.ID, "error", pubErr)
		}
		return pubErr
	}
	next := time.Now().UTC().Add(time.Duration(msg.Attempts) * time.Second)
	if err := store.RescheduleOutbox(persistCtx, msg.ID, msg.Attempts, next, pubErr); err != nil && w.Logger != nil {
		w.Logger.Error("outbox retry was not persisted", "id", msg.ID, "error", err)
	}
	return pubErr
}

func (w *OutboxRunner) acknowledgePublished(ctx context.Context, store OutboxStore, id int64) error {
	persistCtx, cancel := persistenceContext(ctx, w.ackTimeout())
	defer cancel()
	return store.AcknowledgeOutbox(persistCtx, id)
}

// persistenceContext returns a bounded context that survives cancellation of
// the parent, so durable acknowledgements and retries still commit during
// shutdown. The budget defaults to 5 seconds and is raised to the runner's
// shutdown timeout when one is configured.
func persistenceContext(parent context.Context, budget ...time.Duration) (context.Context, context.CancelFunc) {
	timeout := 5 * time.Second
	if len(budget) > 0 && budget[0] > 0 {
		timeout = budget[0]
	}
	return context.WithTimeout(context.WithoutCancel(parent), timeout)
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

// ErrDrainTimeout is returned by OutboxRunner.Wait when an in-flight
// publish+acknowledge cycle does not settle within the shutdown budget.
var ErrDrainTimeout = errors.New("outbox runner did not stop in time")
