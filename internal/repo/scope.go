package repo

import (
	"context"
	"database/sql"
	"errors"

	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
)

func (r *Repository) DeviceHousehold(ctx context.Context, id int64) (int64, error) {
	return r.resourceHousehold(ctx, `SELECT household_id FROM devices WHERE id=$1`, id)
}

func (r *Repository) PlanHousehold(ctx context.Context, id int64) (int64, error) {
	return r.resourceHousehold(ctx, `SELECT household_id FROM energy_plans WHERE id=$1`, id)
}

func (r *Repository) AutomationHousehold(ctx context.Context, id int64) (int64, error) {
	return r.resourceHousehold(ctx, `SELECT household_id FROM automations WHERE id=$1`, id)
}

func (r *Repository) resourceHousehold(ctx context.Context, query string, id int64) (int64, error) {
	if id <= 0 {
		return 0, model.ErrInvalid
	}
	var household int64
	err := r.executor(ctx).QueryRowContext(ctx, query, id).Scan(&household)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, model.ErrNotFound
	}
	return household, err
}
