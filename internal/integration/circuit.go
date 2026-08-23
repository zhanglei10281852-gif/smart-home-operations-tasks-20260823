package integration

import (
	"context"
	"errors"
	"sync"
	"time"
)

type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"
	CircuitOpen     CircuitState = "open"
	CircuitHalfOpen CircuitState = "half_open"
)

type Circuit struct {
	mu          sync.Mutex
	state       CircuitState
	failures    int
	threshold   int
	openedAt    time.Time
	reopenAfter time.Duration
}

func NewCircuit(threshold int, reopenAfter time.Duration) *Circuit {
	if threshold < 1 {
		threshold = 3
	}
	if reopenAfter <= 0 {
		reopenAfter = time.Second
	}
	return &Circuit{state: CircuitClosed, threshold: threshold, reopenAfter: reopenAfter}
}

func (c *Circuit) State(now time.Time) CircuitState {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == CircuitOpen && !now.Before(c.openedAt.Add(c.reopenAfter)) {
		c.state = CircuitHalfOpen
	}
	return c.state
}

func (c *Circuit) Call(ctx context.Context, now time.Time, fn func(context.Context) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	state := c.State(now)
	if state == CircuitOpen {
		return errors.New("circuit is open")
	}
	err := fn(ctx)
	c.mu.Lock()
	defer c.mu.Unlock()
	if err == nil {
		c.state = CircuitClosed
		c.failures = 0
		return nil
	}
	c.failures++
	if c.state == CircuitHalfOpen || c.failures >= c.threshold {
		c.state = CircuitOpen
		c.openedAt = now
	}
	return err
}
