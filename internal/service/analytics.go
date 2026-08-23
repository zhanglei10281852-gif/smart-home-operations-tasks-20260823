package service

import (
	"context"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/analytics"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"time"
)

type AnalyticsService struct {
	Repo interface {
		TelemetryWindow(context.Context, int64, time.Time, time.Time) ([]model.Telemetry, error)
	}
}

func NewAnalytics(r interface {
	TelemetryWindow(context.Context, int64, time.Time, time.Time) ([]model.Telemetry, error)
}) *AnalyticsService {
	return &AnalyticsService{Repo: r}
}
func (s *AnalyticsService) DeviceSummary(ctx context.Context, id int64, start, end time.Time) (analytics.Summary, error) {
	rows, err := s.Repo.TelemetryWindow(ctx, id, start, end)
	if err != nil {
		return analytics.Summary{}, err
	}
	samples := make([]analytics.Sample, len(rows))
	for i, row := range rows {
		samples[i] = analytics.Sample{DeviceID: row.DeviceID, At: row.MeasuredAt, Watts: row.PowerWatts, Temperature: row.TemperatureC}
	}
	return analytics.Summarize(samples), nil
}
func (s *AnalyticsService) Buckets(ctx context.Context, id int64, start, end time.Time, step time.Duration) ([]analytics.Bucket, error) {
	summary, err := s.DeviceSummary(ctx, id, start, end)
	if err != nil {
		return nil, err
	}
	_ = summary
	rows, err := s.Repo.TelemetryWindow(ctx, id, start, end)
	if err != nil {
		return nil, err
	}
	samples := make([]analytics.Sample, len(rows))
	for i, row := range rows {
		samples[i] = analytics.Sample{DeviceID: row.DeviceID, At: row.MeasuredAt, Watts: row.PowerWatts}
	}
	return analytics.Bucketize(samples, start, end, step), nil
}
