package service

import (
	"context"
	"testing"
	"time"
)

func TestParentCancellationStopsManagedLifecycle(t *testing.T) {
	parent, cancelParent := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	lifecycle := StartLifecycle(parent, func(ctx context.Context) {
		<-ctx.Done()
		close(stopped)
	})
	cancelParent()
	select {
	case <-stopped:
	case <-time.After(200 * time.Millisecond):
		_ = lifecycle.Stop(time.Second)
		t.Fatal("managed component remained active after parent cancellation")
	}
	if err := lifecycle.Stop(time.Second); err != nil {
		t.Fatalf("repeated explicit stop failed after parent shutdown: %v", err)
	}
}
