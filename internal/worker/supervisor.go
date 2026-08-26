package worker

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

type Job struct {
	Name     string
	Run      func(context.Context) error
	Interval time.Duration
}
type Supervisor struct {
	Jobs   []Job
	Logger *slog.Logger
	wg     sync.WaitGroup
}

func (s *Supervisor) Start(ctx context.Context) {
	for _, job := range s.Jobs {
		j := job
		if j.Run == nil {
			if s.Logger != nil {
				s.Logger.Error("supervised job is not configured", "job", j.Name)
			}
			continue
		}
		if j.Interval <= 0 {
			j.Interval = time.Minute
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			ticker := time.NewTicker(j.Interval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					launchSupervisedJob(ctx, j, s.Logger)
				}
			}
		}()
	}
}
func (s *Supervisor) Wait() { s.wg.Wait() }
func RunOnce(ctx context.Context, jobs []Job) error {
	for _, job := range jobs {
		if job.Run == nil {
			return errors.New("supervised job is not configured")
		}
		if err := job.Run(ctx); err != nil {
			return err
		}
	}
	return nil
}
