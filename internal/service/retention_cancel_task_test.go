package service

import (
	"context"
	"testing"
	"time"
)

type canceledRetentionRepo struct {
	telemetryDone chan struct{}
	cleanupStart  chan struct{}
	cleanupDone   chan struct{}
}

func (r *canceledRetentionRepo) DeleteTelemetryBefore(context.Context, time.Time) (int64, error) {
	close(r.telemetryDone)
	return 3, nil
}
func (r *canceledRetentionRepo) DeleteExpiredSessions(ctx context.Context, _ time.Time) (int64, error) {
	close(r.cleanupStart)
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-time.After(10 * time.Millisecond):
		close(r.cleanupDone)
		return 4, nil
	}
}

func TestCanceledRetentionDoesNotRunSessionCleanupAfterReturn(t *testing.T) {
	repository := &canceledRetentionRepo{telemetryDone: make(chan struct{}), cleanupStart: make(chan struct{}), cleanupDone: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	service := NewRetention(repository, FixedClock{Value: time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)})
	result := make(chan error, 1)
	go func() { _, err := service.Run(ctx, time.Hour, 2*time.Hour); result <- err }()
	<-repository.telemetryDone
	cancel()
	if err := <-result; err == nil {
		t.Fatal("canceled retention unexpectedly succeeded")
	}
	select {
	case <-repository.cleanupStart:
		t.Fatal("session cleanup started after the canceled caller returned")
	case <-time.After(20 * time.Millisecond):
	}
	select {
	case <-repository.cleanupDone:
		t.Fatal("session cleanup produced a late durable side effect")
	default:
	}
}
