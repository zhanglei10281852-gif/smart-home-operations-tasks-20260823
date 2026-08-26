package worker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zhanglei10281852-gif/smart-home-operations/internal/service"
)

type task045MaintenanceRepo struct {
	started    chan struct{}
	cancelled  chan struct{}
	finished   chan struct{}
	release    chan struct{}
	startOnce  sync.Once
	cancelOnce sync.Once
	finishOnce sync.Once
	postVacuum atomic.Int64
}

func (r *task045MaintenanceRepo) VacuumSessions(ctx context.Context, _ time.Time) (int64, error) {
	r.startOnce.Do(func() { close(r.started) })
	defer r.finishOnce.Do(func() { close(r.finished) })
	select {
	case <-ctx.Done():
		r.cancelOnce.Do(func() { close(r.cancelled) })
		return 0, ctx.Err()
	case <-r.release:
		return 1, nil
	}
}
func (r *task045MaintenanceRepo) ExpirePlans(context.Context, time.Time) (int64, error) {
	r.postVacuum.Add(1)
	return 1, nil
}
func (r *task045MaintenanceRepo) FinishStaleRuns(context.Context, time.Time) (int64, error) {
	r.postVacuum.Add(1)
	return 1, nil
}

func TestSchedulerShutdownWaitsForMaintenanceCancellation(t *testing.T) {
	repository := &task045MaintenanceRepo{
		started:   make(chan struct{}),
		cancelled: make(chan struct{}),
		finished:  make(chan struct{}),
		release:   make(chan struct{}),
	}
	maintenance := service.NewMaintenance(repository, func() time.Time {
		return time.Date(2026, 8, 23, 3, 0, 0, 0, time.UTC)
	})
	scheduler := &Scheduler{Maintenance: maintenance, Interval: 10 * time.Millisecond}
	ctx, cancel := context.WithCancel(context.Background())
	returned := make(chan error, 1)
	go func() { returned <- scheduler.Run(ctx) }()
	<-repository.started
	cancel()

	var runErr error
	deadline := time.NewTimer(time.Second)
	select {
	case runErr = <-returned:
	case <-deadline.C:
		t.Fatal("scheduler did not return after shutdown cancellation")
	}
	if !deadline.Stop() {
		select {
		case <-deadline.C:
		default:
		}
	}
	cancelObserved := false
	select {
	case <-repository.cancelled:
		cancelObserved = true
	default:
	}
	operationFinished := false
	select {
	case <-repository.finished:
		operationFinished = true
	default:
	}
	close(repository.release)
	<-repository.finished

	if !errors.Is(runErr, context.Canceled) {
		t.Fatalf("scheduler shutdown error=%v", runErr)
	}
	if !cancelObserved || !operationFinished || repository.postVacuum.Load() != 0 {
		t.Fatalf("scheduler abandoned maintenance: cancel_observed=%v operation_finished=%v post_cancel_steps=%d", cancelObserved, operationFinished, repository.postVacuum.Load())
	}
}
