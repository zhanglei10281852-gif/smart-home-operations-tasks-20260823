package domain

import (
	"context"
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

func TestTimedOutHealthChecksReleaseLateResultWorkers(t *testing.T) {
	before := runtime.NumGoroutine()
	release := make(chan struct{})
	var callbacksReturned atomic.Int32
	checks := make([]Check, 16)
	for i := range checks {
		checks[i] = Check{
			Name:    fmt.Sprintf("dependency-%d", i),
			Timeout: time.Millisecond,
			Run: func(context.Context) error {
				defer callbacksReturned.Add(1)
				<-release
				return nil
			},
		}
	}
	statuses := RunChecks(context.Background(), checks)
	for _, status := range statuses {
		if status.Healthy || status.Error == "" {
			t.Fatalf("timed out status=%+v", status)
		}
	}
	close(release)
	deadline := time.Now().Add(time.Second)
	for callbacksReturned.Load() != int32(len(checks)) && time.Now().Before(deadline) {
		runtime.Gosched()
	}
	if callbacksReturned.Load() != int32(len(checks)) {
		t.Fatalf("callbacks returned=%d want=%d", callbacksReturned.Load(), len(checks))
	}
	runtime.GC()
	after := runtime.NumGoroutine()
	if leaked := after - before; leaked >= len(checks)-2 {
		t.Fatalf("timed-out checks left %d result workers blocked after callbacks returned", leaked)
	}
}
