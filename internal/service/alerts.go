package service

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/domain"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/repo"
	"time"
)

type AlertService struct {
	Repo  *repo.Repository
	Clock model.Clock
}

func NewAlerts(r *repo.Repository, c model.Clock) *AlertService {
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
	if id <= 0 {
		return model.ErrInvalid
	}
	return errors.New("alert repository is required for resolution")
}
func (s *AlertService) Decision(policy domain.AlertPolicy, value float64, last *time.Time, open bool) domain.AlertDecision {
	return domain.EvaluateAlert(policy, value, last, s.Clock.Now(), open)
}
