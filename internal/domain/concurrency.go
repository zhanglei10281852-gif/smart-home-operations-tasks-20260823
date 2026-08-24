package domain

import (
	"context"
	"errors"
	"sync"
)

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

// runConcurrentJoinAll runs every task to completion in its own goroutine and
// returns the aggregated error. Unlike a fail-fast runner it never abandons an
// in-flight task: one subscriber returning early while another has already
// started cannot make the caller return before the slow subscriber finishes.
// All errors are joined with errors.Join so callers see the full failure set
// rather than only the first error to arrive.
func runConcurrentJoinAll(count int, run func(int) error) error {
	if count <= 0 {
		return nil
	}
	errs := make([]error, count)
	var wg sync.WaitGroup
	for index := 0; index < count; index++ {
		index := index
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[index] = run(index)
		}()
	}
	wg.Wait()
	return errors.Join(errs...)
}
