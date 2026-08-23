package service

import (
	"context"
	"errors"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"strings"
	"time"
)

type AutomationService struct {
	Repo interface {
		CreateAutomation(context.Context, model.Automation, []model.AutomationAction) (model.Automation, error)
		QueueRun(context.Context, int64, string) (model.AutomationRun, error)
		FinishRun(context.Context, int64, model.RunState, string, time.Time) error
	}
	Clock model.Clock
}

func NewAutomation(r interface {
	CreateAutomation(context.Context, model.Automation, []model.AutomationAction) (model.Automation, error)
	QueueRun(context.Context, int64, string) (model.AutomationRun, error)
	FinishRun(context.Context, int64, model.RunState, string, time.Time) error
}, c model.Clock) *AutomationService {
	return &AutomationService{Repo: r, Clock: c}
}
func (s *AutomationService) Create(ctx context.Context, a model.Automation, actions []model.AutomationAction) (model.Automation, error) {
	a.Name = strings.TrimSpace(a.Name)
	if a.HouseholdID <= 0 || a.Name == "" || a.TriggerKind == "" || len(actions) == 0 {
		return model.Automation{}, model.ErrInvalid
	}
	seen := map[int]bool{}
	for _, action := range actions {
		if action.DeviceID <= 0 || action.Action == "" || seen[action.Ordinal] {
			return model.Automation{}, model.ErrInvalid
		}
		seen[action.Ordinal] = true
	}
	return s.Repo.CreateAutomation(ctx, a, actions)
}
func (s *AutomationService) Queue(ctx context.Context, automationID int64, key string) (model.AutomationRun, error) {
	if automationID <= 0 || strings.TrimSpace(key) == "" {
		return model.AutomationRun{}, model.ErrInvalid
	}
	return s.Repo.QueueRun(ctx, automationID, key)
}
func (s *AutomationService) Execute(ctx context.Context, runID int64) error {
	if runID <= 0 {
		return model.ErrInvalid
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return s.Repo.FinishRun(ctx, runID, model.RunSucceeded, "", s.Clock.Now())
}
func (s *AutomationService) ValidateAction(kind model.DeviceKind, action string) error {
	switch kind {
	case model.KindLight:
		if action != "on" && action != "off" {
			return model.ErrInvalid
		}
	case model.KindThermostat:
		if action != "heat" && action != "cool" {
			return model.ErrInvalid
		}
	case model.KindLock:
		if action != "lock" && action != "unlock" {
			return model.ErrInvalid
		}
	default:
		return errors.New("device kind is not commandable")
	}
	return nil
}

type RetryPolicy struct {
	Limit int
	Base  time.Duration
}

func (p RetryPolicy) Delay(attempt int) time.Duration {
	if attempt < 1 {
		return p.Base
	}
	if attempt > 10 {
		attempt = 10
	}
	return p.Base * time.Duration(1<<(attempt-1))
}
