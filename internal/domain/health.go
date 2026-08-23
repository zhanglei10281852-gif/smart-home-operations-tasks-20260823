package domain

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Check struct {
	Name    string
	Run     func(context.Context) error
	Timeout time.Duration
}
type Status struct {
	Name     string
	Healthy  bool
	Duration time.Duration
	Error    string
}

func RunChecks(ctx context.Context, checks []Check) []Status {
	out := make([]Status, 0, len(checks))
	for _, check := range checks {
		start := time.Now()
		checkCtx := ctx
		cancel := func() {}
		if check.Timeout > 0 {
			checkCtx, cancel = context.WithTimeout(ctx, check.Timeout)
		}
		var err error
		if check.Run == nil {
			err = fmt.Errorf("health check %q is not configured", check.Name)
		} else {
			result := startHealthCheck(checkCtx, check.Run)
			select {
			case err = <-result:
			case <-checkCtx.Done():
				err = checkCtx.Err()
			}
		}
		cancel()
		status := Status{Name: check.Name, Healthy: err == nil, Duration: time.Since(start)}
		if err != nil {
			status.Error = err.Error()
		}
		out = append(out, status)
	}
	return out
}

type Semaphore struct {
	slots chan struct{}
	once  sync.Once
}

func NewSemaphore(n int) *Semaphore {
	if n < 1 {
		n = 1
	}
	return &Semaphore{slots: make(chan struct{}, n)}
}
func (s *Semaphore) Acquire(ctx context.Context) error {
	select {
	case s.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("acquire semaphore: %w", ctx.Err())
	}
}
func (s *Semaphore) Release() {
	s.once.Do(func() {})
	select {
	case <-s.slots:
	default:
	}
}
