package service

import (
	"context"
	"fmt"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"strings"
	"time"
)

type EnergyService struct {
	Repo interface {
		CreatePlan(context.Context, model.EnergyPlan, []model.PlanDevice) (model.EnergyPlan, error)
		GetPlan(context.Context, int64) (model.EnergyPlan, error)
		SetPlanState(context.Context, int64, model.PlanState, model.PlanState) error
	}
	Clock model.Clock
}

func NewEnergy(r interface {
	CreatePlan(context.Context, model.EnergyPlan, []model.PlanDevice) (model.EnergyPlan, error)
	GetPlan(context.Context, int64) (model.EnergyPlan, error)
	SetPlanState(context.Context, int64, model.PlanState, model.PlanState) error
}, c model.Clock) *EnergyService {
	return &EnergyService{Repo: r, Clock: c}
}
func (s *EnergyService) Draft(ctx context.Context, p model.EnergyPlan, devices []model.PlanDevice) (model.EnergyPlan, error) {
	p.Name = strings.TrimSpace(p.Name)
	if p.HouseholdID <= 0 || p.Name == "" || p.BudgetCents < 0 || !p.StartsAt.Before(p.EndsAt) || !s.Clock.Now().Before(p.StartsAt) || p.EndsAt.Sub(p.StartsAt) > 31*24*time.Hour {
		return model.EnergyPlan{}, model.ErrInvalid
	}
	if len(devices) == 0 {
		return model.EnergyPlan{}, fmt.Errorf("%w: plan needs devices", model.ErrInvalid)
	}
	seen := make(map[int64]struct{}, len(devices))
	for _, d := range devices {
		if d.DeviceID <= 0 || d.TargetWatts < 0 {
			return model.EnergyPlan{}, model.ErrInvalid
		}
		if _, ok := seen[d.DeviceID]; ok {
			return model.EnergyPlan{}, fmt.Errorf("%w: plan contains a device more than once", model.ErrInvalid)
		}
		seen[d.DeviceID] = struct{}{}
	}
	return s.Repo.CreatePlan(ctx, p, devices)
}
func (s *EnergyService) Schedule(ctx context.Context, id int64) error {
	plan, err := s.Repo.GetPlan(ctx, id)
	if err != nil {
		return err
	}
	if plan.State != model.PlanDraft || !s.Clock.Now().Before(plan.StartsAt) {
		return model.ErrConflict
	}
	return s.Repo.SetPlanState(ctx, id, model.PlanDraft, model.PlanScheduled)
}
func (s *EnergyService) Start(ctx context.Context, id int64) error {
	plan, err := s.Repo.GetPlan(ctx, id)
	if err != nil {
		return err
	}
	now := s.Clock.Now()
	if plan.State != model.PlanScheduled || now.Before(plan.StartsAt) || !now.Before(plan.EndsAt) {
		return model.ErrConflict
	}
	return s.Repo.SetPlanState(ctx, id, model.PlanScheduled, model.PlanRunning)
}
func (s *EnergyService) Complete(ctx context.Context, id int64) error {
	plan, err := s.Repo.GetPlan(ctx, id)
	if err != nil {
		return err
	}
	if plan.State != model.PlanRunning {
		return model.ErrConflict
	}
	return s.Repo.SetPlanState(ctx, id, model.PlanRunning, model.PlanCompleted)
}
func (s *EnergyService) Cancel(ctx context.Context, id int64) error {
	plan, err := s.Repo.GetPlan(ctx, id)
	if err != nil {
		return err
	}
	if plan.State != model.PlanDraft && plan.State != model.PlanScheduled {
		return model.ErrConflict
	}
	return s.Repo.SetPlanState(ctx, id, plan.State, model.PlanCancelled)
}
