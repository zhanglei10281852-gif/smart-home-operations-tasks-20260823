package service

import (
	"context"
	"encoding/json"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/domain"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"time"
)

type AlertService struct {
	Repo interface {
		CreateAlert(context.Context, model.Alert) (model.Alert, error)
		GetAlert(context.Context, int64) (model.Alert, error)
		TransitionAlert(context.Context, int64, string, string, time.Time) error
	}
	Clock model.Clock
}

func NewAlerts(r interface {
	CreateAlert(context.Context, model.Alert) (model.Alert, error)
	GetAlert(context.Context, int64) (model.Alert, error)
	TransitionAlert(context.Context, int64, string, string, time.Time) error
}, c model.Clock) *AlertService {
	return &AlertService{Repo: r, Clock: c}
}

type AlertInput struct {
	HouseholdID    int64
	DeviceID       *int64
	Severity, Code string
	Details        map[string]any
}

func (s *AlertService) Validate(in AlertInput) error {
	if in.HouseholdID <= 0 || in.Code == "" || !domain.ValidSeverity(in.Severity) {
		return model.ErrInvalid
	}
	_, err := json.Marshal(in.Details)
	return err
}
func (s *AlertService) Resolve(ctx context.Context, id int64) error {
	if id <= 0 || s.Repo == nil || s.Clock == nil {
		return model.ErrInvalid
	}
	alert, err := s.Repo.GetAlert(ctx, id)
	if err != nil {
		return err
	}
	if !domain.TransitionAlert(alert.State, "resolved") {
		return model.ErrConflict
	}
	return s.Repo.TransitionAlert(ctx, id, alert.State, "resolved", s.Clock.Now())
}
func (s *AlertService) Decision(policy domain.AlertPolicy, value float64, last *time.Time, open bool) domain.AlertDecision {
	if s == nil || s.Clock == nil {
		return domain.AlertDecision{Reason: "alert service is not configured"}
	}
	return domain.EvaluateAlert(policy, value, last, s.Clock.Now(), open)
}
