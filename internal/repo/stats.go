package repo

import (
	"context"
	"fmt"
	"time"
)

type Stats struct{ Households, Members, Devices, Telemetry, Alerts, Plans, Automations int64 }

func (r *Repository) Stats(ctx context.Context) (Stats, error) {
	var s Stats
	queries := []struct {
		name string
		dest *int64
	}{
		{"households", &s.Households}, {"members", &s.Members}, {"devices", &s.Devices},
		{"telemetry", &s.Telemetry}, {"alerts", &s.Alerts}, {"energy_plans", &s.Plans},
		{"automations", &s.Automations},
	}
	for _, query := range queries {
		if err := r.DB.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+query.name).Scan(query.dest); err != nil {
			return Stats{}, fmt.Errorf("count %s: %w", query.name, err)
		}
	}
	return s, nil
}
func (r *Repository) DeleteTelemetryBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := r.DB.SQL.ExecContext(ctx, `DELETE FROM telemetry WHERE measured_at < $1`, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
func (r *Repository) HealthSnapshot(ctx context.Context) (map[string]any, error) {
	if err := r.DB.SQL.PingContext(ctx); err != nil {
		return nil, err
	}
	stats, err := r.Stats(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{"database": "ok", "stats": stats}, nil
}
