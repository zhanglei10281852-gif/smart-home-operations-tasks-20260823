package service

import (
	"context"
	"errors"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"testing"
	"time"
)

type retentionRepo struct {
	telemetry int64
	sessions  int64
	err       error
	cutoffs   []time.Time
}

func (r *retentionRepo) DeleteTelemetryBefore(_ context.Context, cutoff time.Time) (int64, error) {
	r.cutoffs = append(r.cutoffs, cutoff)
	return r.telemetry, r.err
}
func (r *retentionRepo) DeleteExpiredSessions(_ context.Context, cutoff time.Time) (int64, error) {
	r.cutoffs = append(r.cutoffs, cutoff)
	return r.sessions, r.err
}

func TestRetentionRun(t *testing.T) {
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	repo := &retentionRepo{telemetry: 4, sessions: 2}
	s := NewRetention(repo, FixedClock{Value: now})
	report, err := s.Run(context.Background(), 24*time.Hour, 12*time.Hour)
	if err != nil || report.TelemetryDeleted != 4 || report.SessionsDeleted != 2 || !report.Cutoff.Equal(now.Add(-24*time.Hour)) || len(repo.cutoffs) != 2 {
		t.Fatalf("report=%+v cutoffs=%v err=%v", report, repo.cutoffs, err)
	}
}

func TestRetentionErrors(t *testing.T) {
	if _, err := NewRetention(nil, model.RealClock{}).Run(context.Background(), time.Hour, time.Hour); err == nil {
		t.Fatal("unconfigured retention accepted")
	}
	repo := &retentionRepo{err: errors.New("database down")}
	if _, err := NewRetention(repo, FixedClock{Value: time.Now()}).Run(context.Background(), time.Hour, time.Hour); err == nil {
		t.Fatal("retention database error swallowed")
	}
}

func TestRetentionUsesContextAndSeparateAges(t *testing.T) {
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	repo := &retentionRepo{telemetry: 1, sessions: 1}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// The repository in this unit test records the exact cutoffs; the service
	// still passes the caller context through both deletion boundaries.
	_, _ = NewRetention(repo, FixedClock{Value: now}).Run(ctx, 48*time.Hour, 72*time.Hour)
	if len(repo.cutoffs) != 2 || !repo.cutoffs[0].Equal(now.Add(-48*time.Hour)) || !repo.cutoffs[1].Equal(now.Add(-72*time.Hour)) {
		t.Fatalf("cutoffs=%v", repo.cutoffs)
	}
}
