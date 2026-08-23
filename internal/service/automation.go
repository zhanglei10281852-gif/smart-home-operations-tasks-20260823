package service

import (
	"context"
	"fmt"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"strings"
	"time"
)

type AutomationService struct {
	Repo interface {
		CreateAutomation(context.Context, model.Automation, []model.AutomationAction) (model.Automation, error)
		GetAutomation(context.Context, int64) (model.Automation, error)
		GetDevice(context.Context, int64) (model.Device, error)
		SetAutomationState(context.Context, int64, model.AutomationState, model.AutomationState) error
		QueueRun(context.Context, int64, string) (model.AutomationRun, error)
		ExecuteAutomationRun(context.Context, int64, time.Time) error
	}
	Clock model.Clock
}

func NewAutomation(r interface {
	CreateAutomation(context.Context, model.Automation, []model.AutomationAction) (model.Automation, error)
	GetAutomation(context.Context, int64) (model.Automation, error)
	GetDevice(context.Context, int64) (model.Device, error)
	SetAutomationState(context.Context, int64, model.AutomationState, model.AutomationState) error
	QueueRun(context.Context, int64, string) (model.AutomationRun, error)
	ExecuteAutomationRun(context.Context, int64, time.Time) error
}, c model.Clock) *AutomationService {
	return &AutomationService{Repo: r, Clock: c}
}
func (s *AutomationService) Create(ctx context.Context, a model.Automation, actions []model.AutomationAction) (model.Automation, error) {
	a.Name = strings.TrimSpace(a.Name)
	a.TriggerKind = strings.TrimSpace(a.TriggerKind)
	if a.HouseholdID <= 0 || a.Name == "" || a.TriggerKind == "" || len(actions) == 0 {
		return model.Automation{}, model.ErrInvalid
	}
	seen := map[int]bool{}
	for i := range actions {
		action := actions[i]
		action.Action = strings.TrimSpace(action.Action)
		actions[i].Action = action.Action
		if action.DeviceID <= 0 || action.Action == "" || action.Ordinal < 0 || seen[action.Ordinal] {
			return model.Automation{}, model.ErrInvalid
		}
		device, err := s.Repo.GetDevice(ctx, action.DeviceID)
		if err != nil {
			return model.Automation{}, err
		}
		if device.HouseholdID != a.HouseholdID || device.State != model.DeviceEnabled {
			return model.Automation{}, model.ErrConflict
		}
		if err := s.ValidateAction(device.Kind, action.Action); err != nil {
			return model.Automation{}, err
		}
		seen[action.Ordinal] = true
	}
	return s.Repo.CreateAutomation(ctx, a, actions)
}
func (s *AutomationService) Queue(ctx context.Context, automationID int64, key string) (model.AutomationRun, error) {
	if automationID <= 0 || strings.TrimSpace(key) == "" {
		return model.AutomationRun{}, model.ErrInvalid
	}
	automation, err := s.Repo.GetAutomation(ctx, automationID)
	if err != nil {
		return model.AutomationRun{}, err
	}
	if automation.State != model.AutomationActive {
		return model.AutomationRun{}, model.ErrConflict
	}
	return s.Repo.QueueRun(ctx, automationID, key)
}
func (s *AutomationService) Activate(ctx context.Context, automationID int64) error {
	if automationID <= 0 {
		return model.ErrInvalid
	}
	return s.Repo.SetAutomationState(ctx, automationID, model.AutomationDraft, model.AutomationActive)
}
func (s *AutomationService) Pause(ctx context.Context, automationID int64) error {
	if automationID <= 0 {
		return model.ErrInvalid
	}
	return s.Repo.SetAutomationState(ctx, automationID, model.AutomationActive, model.AutomationPaused)
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
	return s.Repo.ExecuteAutomationRun(ctx, runID, s.Clock.Now())
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
		return fmt.Errorf("%w: device kind is not commandable", model.ErrInvalid)
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
