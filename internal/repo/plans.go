package repo

import (
	"context"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"time"
)

func (r *Repository) GetPlan(ctx context.Context, id int64) (model.EnergyPlan, error) {
	var p model.EnergyPlan
	err := r.executor(ctx).QueryRowContext(ctx, `SELECT id,household_id,name,state,budget_cents,starts_at,ends_at,created_at FROM energy_plans WHERE id=$1`, id).Scan(&p.ID, &p.HouseholdID, &p.Name, &p.State, &p.BudgetCents, &p.StartsAt, &p.EndsAt, &p.CreatedAt)
	return p, err
}
func (r *Repository) ListPlans(ctx context.Context, household int64, state model.PlanState) ([]model.EnergyPlan, error) {
	query := `SELECT id,household_id,name,state,budget_cents,starts_at,ends_at,created_at FROM energy_plans WHERE household_id=$1`
	args := []any{household}
	if state != "" {
		query += ` AND state=$2`
		args = append(args, state)
	}
	query += ` ORDER BY starts_at DESC`
	rows, err := r.executor(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.EnergyPlan{}
	for rows.Next() {
		var p model.EnergyPlan
		if err := rows.Scan(&p.ID, &p.HouseholdID, &p.Name, &p.State, &p.BudgetCents, &p.StartsAt, &p.EndsAt, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (r *Repository) PlanTotalWatts(ctx context.Context, id int64) (float64, error) {
	var total float64
	err := r.executor(ctx).QueryRowContext(ctx, `SELECT COALESCE(SUM(target_watts),0) FROM plan_devices WHERE plan_id=$1`, id).Scan(&total)
	return total, err
}
func (r *Repository) ExpirePlans(ctx context.Context, now time.Time) (int64, error) {
	res, err := r.executor(ctx).ExecContext(ctx, `UPDATE energy_plans SET state='cancelled' WHERE state IN ('draft','scheduled') AND ends_at < $1`, now)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
