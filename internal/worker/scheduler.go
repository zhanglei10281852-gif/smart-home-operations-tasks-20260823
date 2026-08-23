package worker

import (
	"context"
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
			if _, err := s.Maintenance.Run(ctx); err != nil && s.Logger != nil {
				s.Logger.Error("maintenance cycle failed", "error", err)
			}
		}
	}
}
func (s *Scheduler) RunOnce(ctx context.Context) (service.MaintenanceReport, error) {
	return s.Maintenance.Run(ctx)
}
