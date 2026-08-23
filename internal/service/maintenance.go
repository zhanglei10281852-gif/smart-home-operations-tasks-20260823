package service

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type MaintenanceService struct {
	Repo interface {
		VacuumSessions(context.Context, time.Time) (int64, error)
		ExpirePlans(context.Context, time.Time) (int64, error)
		FinishStaleRuns(context.Context, time.Time) (int64, error)
	}
	Clock func() time.Time
}

func NewMaintenance(r interface {
	VacuumSessions(context.Context, time.Time) (int64, error)
	ExpirePlans(context.Context, time.Time) (int64, error)
	FinishStaleRuns(context.Context, time.Time) (int64, error)
}, clock func() time.Time) *MaintenanceService {
	return &MaintenanceService{Repo: r, Clock: clock}
}

type MaintenanceReport struct{ SessionsDeleted, PlansExpired, RunsFailed int64 }

func (s *MaintenanceService) Run(ctx context.Context) (MaintenanceReport, error) {
	if s == nil || s.Repo == nil || s.Clock == nil {
		return MaintenanceReport{}, errors.New("maintenance is not configured")
	}
	now := s.Clock()
	if now.IsZero() {
		return MaintenanceReport{}, errors.New("maintenance clock returned zero time")
	}
	cycleCtx := context.WithoutCancel(ctx)
	sessions, err := s.Repo.VacuumSessions(cycleCtx, now)
	if err != nil {
		return MaintenanceReport{}, fmt.Errorf("vacuum sessions: %w", err)
	}
	plans, err := s.Repo.ExpirePlans(cycleCtx, now)
	if err != nil {
		return MaintenanceReport{}, fmt.Errorf("expire plans: %w", err)
	}
	runs, err := s.Repo.FinishStaleRuns(cycleCtx, now.Add(-15*time.Minute))
	if err != nil {
		return MaintenanceReport{}, fmt.Errorf("finish stale runs: %w", err)
	}
	return MaintenanceReport{SessionsDeleted: sessions, PlansExpired: plans, RunsFailed: runs}, nil
}
