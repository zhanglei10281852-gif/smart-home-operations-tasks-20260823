package domain

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// BatchItem describes one device command in an operational batch.
type BatchItem struct {
	DeviceID int64
	Action   string
	Payload  map[string]any
}

type BatchResult struct {
	DeviceID int64
	Accepted bool
	Error    error
	Started  time.Time
	Finished time.Time
}

// ValidateBatch rejects duplicate devices and malformed commands before any side effect.
func ValidateBatch(items []BatchItem) error {
	if len(items) == 0 || len(items) > 100 {
		return errors.New("batch must contain between one and one hundred items")
	}
	seen := make(map[int64]struct{}, len(items))
	for _, item := range items {
		if item.DeviceID <= 0 || item.Action == "" {
			return errors.New("batch item is incomplete")
		}
		if _, ok := seen[item.DeviceID]; ok {
			return fmt.Errorf("device %d appears more than once", item.DeviceID)
		}
		seen[item.DeviceID] = struct{}{}
	}
	return nil
}

// ExecuteBatch runs independent device operations concurrently while preserving
// deterministic result ordering and returning cancellation as an item error.
func ExecuteBatch(ctx context.Context, items []BatchItem, execute func(context.Context, BatchItem) error) ([]BatchResult, error) {
	if err := ValidateBatch(items); err != nil {
		return nil, err
	}
	if execute == nil {
		return nil, errors.New("batch executor is not configured")
	}
	results := make([]BatchResult, len(items))
	var wg sync.WaitGroup
	var firstErr error
	var errMu sync.Mutex
	for i := range items {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := BatchResult{DeviceID: items[i].DeviceID, Started: time.Now().UTC()}
			var err error
			select {
			case <-ctx.Done():
				err = ctx.Err()
			default:
				err = execute(ctx, items[i])
			}
			result.Finished = time.Now().UTC()
			result.Error = err
			result.Accepted = err == nil
			results[i] = result
			if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				errMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				errMu.Unlock()
			}
		}()
	}
	// Wait for every in-flight operation to settle before returning. Even when
	// the context is cancelled after work has started, the caller must receive a
	// complete, stable per-device slice; returning early on ctx.Done() would
	// hand back zero-value entries while the goroutines keep writing results in
	// the background (a data race on top of incomplete output).
	wg.Wait()
	if ctx.Err() != nil {
		return results, ctx.Err()
	}
	return results, firstErr
}

// SortBatchResults orders results for API responses without mutating callers.
func SortBatchResults(results []BatchResult) []BatchResult {
	out := append([]BatchResult(nil), results...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].DeviceID < out[j].DeviceID })
	return out
}
