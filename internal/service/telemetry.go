package service

import (
	"context"
	"fmt"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"time"
)

type TelemetryService struct {
	Repo interface {
		InsertTelemetry(context.Context, model.Telemetry) error
		TelemetryWindow(context.Context, int64, time.Time, time.Time) ([]model.Telemetry, error)
	}
	Clock model.Clock
}

func NewTelemetry(r interface {
	InsertTelemetry(context.Context, model.Telemetry) error
	TelemetryWindow(context.Context, int64, time.Time, time.Time) ([]model.Telemetry, error)
}, c model.Clock) *TelemetryService {
	return &TelemetryService{Repo: r, Clock: c}
}
func (s *TelemetryService) Record(ctx context.Context, t model.Telemetry) error {
	if t.DeviceID <= 0 || t.Sequence <= 0 || t.MeasuredAt.IsZero() {
		return model.ErrInvalid
	}
	if t.PowerWatts < 0 {
		return fmt.Errorf("%w: power must be non-negative", model.ErrInvalid)
	}
	if t.MeasuredAt.After(s.Clock.Now().Add(5 * time.Minute)) {
		return fmt.Errorf("%w: measurement is in the future", model.ErrInvalid)
	}
	return s.Repo.InsertTelemetry(ctx, t)
}
func (s *TelemetryService) Window(ctx context.Context, deviceID int64, start, end time.Time) ([]model.Telemetry, error) {
	if deviceID <= 0 || start.IsZero() || end.IsZero() || !start.Before(end) {
		return nil, model.ErrInvalid
	}
	if end.Sub(start) > 7*24*time.Hour {
		return nil, fmt.Errorf("%w: window exceeds seven days", model.ErrInvalid)
	}
	return s.Repo.TelemetryWindow(ctx, deviceID, start, end)
}
func AveragePower(rows []model.Telemetry) float64 {
	if len(rows) == 0 {
		return 0
	}
	var total float64
	for _, row := range rows {
		total += row.PowerWatts
	}
	return total / float64(len(rows))
}
