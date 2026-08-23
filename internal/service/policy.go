package service

import (
	"context"
	"errors"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/domain"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"time"
)

type PolicyService struct{ Clock model.Clock }

func NewPolicy(c model.Clock) *PolicyService { return &PolicyService{Clock: c} }
func (s *PolicyService) ValidateSchedule(ctx context.Context, windows []domain.ScheduleWindow, limit float64) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if len(windows) == 0 {
		return errors.New("schedule is empty")
	}
	if err := domain.Capacity(windows, limit); err != nil {
		return err
	}
	return nil
}
func (s *PolicyService) ValidatePlan(start, end time.Time) error {
	if s == nil || s.Clock == nil {
		return errors.New("policy clock is not configured")
	}
	return domain.ValidatePlanWindow(start, end, s.Clock.Now())
}
