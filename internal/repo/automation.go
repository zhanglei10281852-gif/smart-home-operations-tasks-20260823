package repo

import (
	"context"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"time"
)

func (r *Repository) GetAutomation(ctx context.Context, id int64) (model.Automation, error) {
	var a model.Automation
	err := r.executor(ctx).QueryRowContext(ctx, `SELECT id,household_id,name,state,trigger_kind,created_at FROM automations WHERE id=$1`, id).Scan(&a.ID, &a.HouseholdID, &a.Name, &a.State, &a.TriggerKind, &a.CreatedAt)
	return a, err
}
func (r *Repository) Actions(ctx context.Context, id int64) ([]model.AutomationAction, error) {
	rows, err := r.executor(ctx).QueryContext(ctx, `SELECT id,automation_id,device_id,action,ordinal FROM automation_actions WHERE automation_id=$1 ORDER BY ordinal`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.AutomationAction{}
	for rows.Next() {
		var a model.AutomationAction
		if err := rows.Scan(&a.ID, &a.AutomationID, &a.DeviceID, &a.Action, &a.Ordinal); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func (r *Repository) SetAutomationState(ctx context.Context, id int64, from, to model.AutomationState) error {
	res, err := r.executor(ctx).ExecContext(ctx, `UPDATE automations SET state=$2 WHERE id=$1 AND state=$3`, id, to, from)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return model.ErrConflict
	}
	return nil
}
func (r *Repository) FinishStaleRuns(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := r.executor(ctx).ExecContext(ctx, `UPDATE automation_runs SET state='failed',error_text='worker timeout',finished_at=now() WHERE state='running' AND started_at < $1`, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
