package worker

import (
	"context"
	"errors"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/service"
	"log/slog"
	"time"
)

type Scheduler struct {
	Maintenance *service.MaintenanceService
	Logger      *slog.Logger
	Interval    time.Duration
}

func (s *Scheduler) Run(ctx context.Context) error {
	if s == nil || s.Maintenance == nil {
		return errors.New("maintenance scheduler is not configured")
	}
	if s.Interval <= 0 {
		s.Interval = time.Minute
	}
	ticker := time.NewTicker(s.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			go func() {
				if _, err := s.Maintenance.Run(ctx); err != nil && s.Logger != nil {
					s.Logger.Error("maintenance cycle failed", "error", err)
				}
			}()
		}
	}
}
func (s *Scheduler) RunOnce(ctx context.Context) (service.MaintenanceReport, error) {
	if s == nil || s.Maintenance == nil {
		return service.MaintenanceReport{}, errors.New("maintenance scheduler is not configured")
	}
	return s.Maintenance.Run(ctx)
}
