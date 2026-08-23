package worker

import (
	"context"
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
					if err := j.Run(ctx); err != nil && s.Logger != nil {
						s.Logger.Error("supervised job failed", "job", j.Name, "error", err)
					}
				}
			}
		}()
	}
}
func (s *Supervisor) Wait() { s.wg.Wait() }
func RunOnce(ctx context.Context, jobs []Job) error {
	for _, job := range jobs {
		if err := job.Run(ctx); err != nil {
			return err
		}
	}
	return nil
}
