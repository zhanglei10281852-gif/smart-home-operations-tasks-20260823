package domain

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type semaphoreWaiter struct {
	ready chan struct{}
}

type waiterQueue struct {
	mu       sync.Mutex
	capacity int
	active   int
	waiters  []*semaphoreWaiter
}

func newWaiterQueue(capacity int) *waiterQueue {
	return &waiterQueue{capacity: capacity}
}

func (q *waiterQueue) acquire(ctx context.Context) error {
	q.mu.Lock()
	if q.active < q.capacity {
		q.active++
		q.mu.Unlock()
		return nil
	}
	waiter := &semaphoreWaiter{ready: make(chan struct{})}
	q.waiters = append(q.waiters, waiter)
	q.mu.Unlock()
	select {
	case <-waiter.ready:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("acquire semaphore: %w", ctx.Err())
	}
}

func (q *waiterQueue) release() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.waiters) > 0 {
		waiter := q.waiters[0]
		q.waiters = q.waiters[1:]
		close(waiter.ready)
		return
	}
	if q.active > 0 {
		q.active--
	}
}

// Parallel executes work with a cancellation-aware worker pool. The returned
// slice always has one entry per input, preserving index correspondence.
func Parallel[T any](ctx context.Context, values []T, fn func(context.Context, T) error) []error {
	workers := len(values)
	if workers < 1 {
		workers = 1
	}
	errs := make([]error, len(values))
	if fn == nil {
		for i := range errs {
			errs[i] = errors.New("parallel callback is not configured")
		}
		return errs
	}
	jobs := make(chan int)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				if err := ctx.Err(); err != nil {
					errs[index] = err
					continue
				}
				errs[index] = fn(ctx, values[index])
			}
		}()
	}
	for i := range values {
		select {
		case jobs <- i:
		case <-ctx.Done():
			errs[i] = ctx.Err()
		}
	}
	close(jobs)
	wg.Wait()
	return errs
}

func FirstError(errs []error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func RequireAll(errs []error) error {
	var combined error
	for _, err := range errs {
		if err != nil {
			combined = errors.Join(combined, err)
		}
	}
	return combined
}
