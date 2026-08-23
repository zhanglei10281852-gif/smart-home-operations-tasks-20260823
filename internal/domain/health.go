package domain

import (
	"context"
	"fmt"
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
			err = check.Run(checkCtx)
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
	queue *waiterQueue
}

func NewSemaphore(n int) *Semaphore {
	if n < 1 {
		n = 1
	}
	return &Semaphore{queue: newWaiterQueue(n)}
}
func (s *Semaphore) Acquire(ctx context.Context) error {
	return s.queue.acquire(ctx)
}
func (s *Semaphore) Release() {
	s.queue.release()
}
