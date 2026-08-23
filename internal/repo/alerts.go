package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"time"
)

func (r *Repository) CreateAlert(ctx context.Context, a model.Alert) (model.Alert, error) {
	if len(a.Details) == 0 {
		a.Details = []byte(`{}`)
	}
	var out model.Alert
	err := r.executor(ctx).QueryRowContext(ctx, `INSERT INTO alerts(household_id,device_id,severity,code,state,details) VALUES($1,$2,$3,$4,'open',$5) RETURNING id,household_id,device_id,severity,code,state,details,created_at`, a.HouseholdID, a.DeviceID, a.Severity, a.Code, a.Details).Scan(&out.ID, &out.HouseholdID, &out.DeviceID, &out.Severity, &out.Code, &out.State, &out.Details, &out.CreatedAt)
	return out, err
}
func (r *Repository) GetAlert(ctx context.Context, id int64) (model.Alert, error) {
	var a model.Alert
	err := r.executor(ctx).QueryRowContext(ctx, `SELECT id,household_id,device_id,severity,code,state,details,created_at,resolved_at FROM alerts WHERE id=$1`, id).Scan(&a.ID, &a.HouseholdID, &a.DeviceID, &a.Severity, &a.Code, &a.State, &a.Details, &a.CreatedAt, &a.ResolvedAt)
	return a, err
}
func (r *Repository) TransitionAlert(ctx context.Context, id int64, from, to string, now time.Time) error {
	res, err := r.executor(ctx).ExecContext(ctx, `UPDATE alerts SET state=$2,resolved_at=CASE WHEN $2='resolved' THEN $3 ELSE resolved_at END WHERE id=$1 AND state=$4`, id, to, now, from)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return model.ErrConflict
	}
	return nil
}
func (r *Repository) ListAlerts(ctx context.Context, household int64, state string) ([]model.Alert, error) {
	query := `SELECT id,household_id,device_id,severity,code,state,details,created_at,resolved_at FROM alerts WHERE household_id=$1`
	args := []any{household}
	if state != "" {
		query += ` AND state=$2`
		args = append(args, state)
	}
	query += ` ORDER BY created_at DESC`
	rows, err := r.executor(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Alert{}
	for rows.Next() {
		var a model.Alert
		if err := rows.Scan(&a.ID, &a.HouseholdID, &a.DeviceID, &a.Severity, &a.Code, &a.State, &a.Details, &a.CreatedAt, &a.ResolvedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func encodePayload(v any) []byte { b, _ := json.Marshal(v); return b }

var _ *sql.DB
