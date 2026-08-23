package service

import (
	"context"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"sort"
	"time"
)

type ReportService struct {
	Repo interface {
		TelemetryWindow(context.Context, int64, time.Time, time.Time) ([]model.Telemetry, error)
	}
}

func NewReport(r interface {
	TelemetryWindow(context.Context, int64, time.Time, time.Time) ([]model.Telemetry, error)
}) *ReportService {
	return &ReportService{Repo: r}
}

type DeviceReport struct {
	DeviceID       int64
	SampleCount    int
	AveragePower   float64
	LastMeasuredAt time.Time
}

func BuildDeviceReport(rows []model.Telemetry) DeviceReport {
	report := DeviceReport{}
	if len(rows) == 0 {
		return report
	}
	report.DeviceID = rows[0].DeviceID
	report.SampleCount = len(rows)
	report.AveragePower = AveragePower(rows)
	for _, row := range rows {
		if row.MeasuredAt.After(report.LastMeasuredAt) {
			report.LastMeasuredAt = row.MeasuredAt
		}
	}
	return report
}
func SortTelemetry(rows []model.Telemetry) []model.Telemetry {
	out := append([]model.Telemetry(nil), rows...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].MeasuredAt.Before(out[j].MeasuredAt) })
	return out
}
func (s *ReportService) Telemetry(ctx context.Context, id int64, start, end time.Time) (DeviceReport, error) {
	rows, err := s.Repo.TelemetryWindow(ctx, id, start, end)
	if err != nil {
		return DeviceReport{}, err
	}
	return BuildDeviceReport(SortTelemetry(rows)), nil
}
