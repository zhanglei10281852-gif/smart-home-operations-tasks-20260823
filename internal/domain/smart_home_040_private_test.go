package domain

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCanceledSemaphoreWaiterDoesNotBlockLiveHandoff(t *testing.T) {
	semaphore := NewSemaphore(1)
	if err := semaphore.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}

	canceledCtx, cancel := context.WithCancel(context.Background())
	canceled := make(chan error, 1)
	go func() { canceled <- semaphore.Acquire(canceledCtx) }()
	for {
		semaphore.queue.mu.Lock()
		queued := len(semaphore.queue.waiters)
		semaphore.queue.mu.Unlock()
		if queued == 1 {
			break
		}
	}
	cancel()
	if err := <-canceled; !errors.Is(err, context.Canceled) {
		t.Fatalf("first waiter err=%v", err)
	}

	liveCtx, cancelLive := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancelLive()
	live := make(chan error, 1)
	go func() { live <- semaphore.Acquire(liveCtx) }()
	for {
		semaphore.queue.mu.Lock()
		queued := len(semaphore.queue.waiters)
		semaphore.queue.mu.Unlock()
		if queued == 2 {
			break
		}
	}
	semaphore.Release()
	if err := <-live; err != nil {
		t.Fatalf("live waiter did not receive released slot after canceled waiter: %v", err)
	}
	semaphore.Release()
}
