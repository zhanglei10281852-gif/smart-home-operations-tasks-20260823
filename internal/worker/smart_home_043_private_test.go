package worker

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestSupervisorDoesNotOverlapSlowJobCycles(t *testing.T) {
	var active atomic.Int32
	var maximum atomic.Int32
	started := make(chan struct{}, 4)
	release := make(chan struct{})
	job := Job{Name: "session-maintenance", Interval: 10 * time.Millisecond, Run: func(context.Context) error {
		current := active.Add(1)
		for {
			previous := maximum.Load()
			if current <= previous || maximum.CompareAndSwap(previous, current) {
				break
			}
		}
		started <- struct{}{}
		<-release
		active.Add(-1)
		return nil
	}}
	supervisor := &Supervisor{Jobs: []Job{job}}
	ctx, cancel := context.WithCancel(context.Background())
	supervisor.Start(ctx)
	<-started
	select {
	case <-started:
		close(release)
		cancel()
		supervisor.Wait()
		t.Fatalf("same maintenance job overlapped; maximum active=%d", maximum.Load())
	case <-time.After(35 * time.Millisecond):
	}
	close(release)
	cancel()
	supervisor.Wait()
	if maximum.Load() != 1 {
		t.Fatalf("maximum concurrent job executions=%d", maximum.Load())
	}
}
