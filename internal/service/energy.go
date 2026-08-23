package service

import (
	"context"
	"errors"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"time"
)

type EnergyService struct {
	Repo interface {
		CreatePlan(context.Context, model.EnergyPlan, []model.PlanDevice) (model.EnergyPlan, error)
		SetPlanState(context.Context, int64, model.PlanState, model.PlanState) error
	}
	Clock model.Clock
}

func NewEnergy(r interface {
	CreatePlan(context.Context, model.EnergyPlan, []model.PlanDevice) (model.EnergyPlan, error)
	SetPlanState(context.Context, int64, model.PlanState, model.PlanState) error
}, c model.Clock) *EnergyService {
	return &EnergyService{Repo: r, Clock: c}
}
func (s *EnergyService) Draft(ctx context.Context, p model.EnergyPlan, devices []model.PlanDevice) (model.EnergyPlan, error) {
	if p.HouseholdID <= 0 || p.Name == "" || p.BudgetCents < 0 || !p.StartsAt.Before(p.EndsAt) || p.EndsAt.Sub(p.StartsAt) > 31*24*time.Hour {
		return model.EnergyPlan{}, model.ErrInvalid
	}
	if len(devices) == 0 {
		return model.EnergyPlan{}, errors.New("plan needs devices")
	}
	var total float64
	for _, d := range devices {
		if d.DeviceID <= 0 || d.TargetWatts < 0 {
			return model.EnergyPlan{}, model.ErrInvalid
		}
		total += d.TargetWatts
	}
	if total > float64(p.BudgetCents) {
		return model.EnergyPlan{}, errors.New("plan exceeds budget")
	}
	return s.Repo.CreatePlan(ctx, p, devices)
}
func (s *EnergyService) Schedule(ctx context.Context, id int64) error {
	return s.Repo.SetPlanState(ctx, id, model.PlanDraft, model.PlanScheduled)
}
func (s *EnergyService) Start(ctx context.Context, id int64) error {
	return s.Repo.SetPlanState(ctx, id, model.PlanScheduled, model.PlanRunning)
}
func (s *EnergyService) Complete(ctx context.Context, id int64) error {
	return s.Repo.SetPlanState(ctx, id, model.PlanRunning, model.PlanCompleted)
}
func (s *EnergyService) Cancel(ctx context.Context, id int64) error {
	if err := s.Repo.SetPlanState(ctx, id, model.PlanDraft, model.PlanCancelled); err == nil {
		return nil
	}
	return s.Repo.SetPlanState(ctx, id, model.PlanScheduled, model.PlanCancelled)
}
