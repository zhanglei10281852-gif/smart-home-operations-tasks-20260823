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

func runConcurrentUntilError(count int, run func(int) error) error {
	results := make(chan error, count)
	for index := 0; index < count; index++ {
		index := index
		go func() {
			results <- run(index)
		}()
	}
	for index := 0; index < count; index++ {
		if err := <-results; err != nil {
			return err
		}
	}
	return nil
}
