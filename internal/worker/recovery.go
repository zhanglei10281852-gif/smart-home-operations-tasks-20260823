package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/service"
)

type RunFailureStore interface {
	RequeueRun(context.Context, int64) error
	FinishRun(context.Context, int64, model.RunState, string, time.Time) error
}

func HandleRunFailure(ctx context.Context, store RunFailureStore, runID int64, runErr error, finishedAt time.Time) error {
	if store == nil || runErr == nil {
		return errors.New("run failure handler is not configured")
	}
	if errors.Is(runErr, context.Canceled) || errors.Is(runErr, context.DeadlineExceeded) {
		requeueErr := store.RequeueRun(ctx, runID)
		if requeueErr == nil {
			return nil
		}
		runErr = service.CombineExecutionErrors(runErr, requeueErr)
		if errors.Is(runErr, context.Canceled) || errors.Is(runErr, context.DeadlineExceeded) {
			return runErr
		}
	}
	return store.FinishRun(ctx, runID, model.RunFailed, runErr.Error(), finishedAt)
}

type RetryState struct {
	Attempt   int
	Limit     int
	Next      time.Time
	LastError string
	Permanent bool
}

func (s RetryState) Validate() error {
	if s.Attempt < 0 || s.Limit < 0 || s.Attempt > s.Limit {
		return errors.New("invalid retry state")
	}
	if s.Attempt > 0 && s.Next.IsZero() {
		return errors.New("retry time is required")
	}
	if s.Permanent && s.Attempt < s.Limit {
		return errors.New("permanent state before retry limit")
	}
	return nil
}

func NextRetry(state RetryState, cause error, now time.Time, backoff func(int) time.Duration) RetryState {
	state.Attempt++
	state.LastError = errorString(cause)
	if state.Attempt >= state.Limit {
		state.Permanent = true
		state.Next = now
		return state
	}
	state.Next = now.Add(backoff(state.Attempt))
	return state
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

type Health struct {
	Name      string
	Healthy   bool
	CheckedAt time.Time
	Error     string
}

type HealthCheck func(context.Context) error

func RunHealthChecks(ctx context.Context, checks map[string]HealthCheck, now func() time.Time) []Health {
	keys := make([]string, 0, len(checks))
	for name := range checks {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	if now == nil {
		now = time.Now
	}
	results := make([]Health, len(keys))
	var wg sync.WaitGroup
	for i, name := range keys {
		i, name := i, name
		wg.Add(1)
		go func() {
			defer wg.Done()
			check := checks[name]
			var err error
			if check == nil {
				err = fmt.Errorf("health check %q is not configured", name)
			} else {
				err = check(ctx)
			}
			results[i] = Health{Name: name, Healthy: err == nil, CheckedAt: now(), Error: errorString(err)}
		}()
	}
	wg.Wait()
	return results
}

type GuardedRunner struct {
	Logger *slog.Logger
	Run    func(context.Context) error
}

func (r GuardedRunner) Serve(ctx context.Context) (err error) {
	if r.Run == nil {
		return errors.New("runner is not configured")
	}
	defer func() {
		if value := recover(); value != nil {
			if r.Logger != nil {
				r.Logger.Error("worker panic recovered", "value", value)
			}
			err = fmt.Errorf("worker panic: %v", value)
		}
	}()
	if err := r.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("worker stopped: %w", err)
	}
	return nil
}
