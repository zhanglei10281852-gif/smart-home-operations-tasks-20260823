package repo

import (
	"context"
	"time"
)

type Stats struct{ Households, Members, Devices, Telemetry, Alerts, Plans, Automations int64 }

func (r *Repository) Stats(ctx context.Context) Stats {
	var s Stats
	_ = r.DB.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM households`).Scan(&s.Households)
	_ = r.DB.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM members`).Scan(&s.Members)
	_ = r.DB.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM devices`).Scan(&s.Devices)
	_ = r.DB.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM telemetry`).Scan(&s.Telemetry)
	_ = r.DB.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM alerts`).Scan(&s.Alerts)
	_ = r.DB.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM energy_plans`).Scan(&s.Plans)
	_ = r.DB.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM automations`).Scan(&s.Automations)
	return s
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
	stats := r.Stats(ctx)
	return map[string]any{"database": "ok", "stats": stats}, nil
}
