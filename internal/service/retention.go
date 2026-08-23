package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
)

type RetentionRepository interface {
	DeleteTelemetryBefore(context.Context, time.Time) (int64, error)
	DeleteExpiredSessions(context.Context, time.Time) (int64, error)
}

type RetentionService struct {
	Repo  RetentionRepository
	Clock model.Clock
}

func NewRetention(repo RetentionRepository, clock model.Clock) *RetentionService {
	return &RetentionService{Repo: repo, Clock: clock}
}

type RetentionReport struct {
	TelemetryDeleted int64
	SessionsDeleted  int64
	Cutoff           time.Time
}

func (s *RetentionService) Run(ctx context.Context, telemetryAge, sessionAge time.Duration) (RetentionReport, error) {
	if s.Repo == nil || telemetryAge <= 0 || sessionAge <= 0 {
		return RetentionReport{}, errors.New("retention is not configured")
	}
	now := s.Clock.Now()
	if now.IsZero() {
		return RetentionReport{}, errors.New("clock returned zero time")
	}
	telemetry, err := s.Repo.DeleteTelemetryBefore(ctx, now.Add(-telemetryAge))
	if err != nil {
		return RetentionReport{}, fmt.Errorf("delete telemetry: %w", err)
	}
	sessions, err := s.Repo.DeleteExpiredSessions(ctx, now.Add(-sessionAge))
	if err != nil {
		return RetentionReport{}, fmt.Errorf("delete sessions: %w", err)
	}
	return RetentionReport{TelemetryDeleted: telemetry, SessionsDeleted: sessions, Cutoff: now.Add(-telemetryAge)}, nil
}
