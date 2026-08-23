package repo

import (
	"context"
	"fmt"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"time"
)

func (r *Repository) GetAutomation(ctx context.Context, id int64) (model.Automation, error) {
	var a model.Automation
	err := r.executor(ctx).QueryRowContext(ctx, `SELECT id,household_id,name,state,trigger_kind,created_at FROM automations WHERE id=$1`, id).Scan(&a.ID, &a.HouseholdID, &a.Name, &a.State, &a.TriggerKind, &a.CreatedAt)
	return a, classifyReadError(err)
}

func (r *Repository) ExecuteAutomationRun(ctx context.Context, runID int64, finishedAt time.Time) error {
	if runID <= 0 || finishedAt.IsZero() {
		return model.ErrInvalid
	}
	return r.DB.WithTx(ctx, func(txctx context.Context) error {
		executor := r.executor(txctx)
		var automationID, householdID int64
		if err := executor.QueryRowContext(txctx, `SELECT runs.automation_id,automations.household_id FROM automation_runs runs JOIN automations ON automations.id=runs.automation_id WHERE runs.id=$1 AND runs.state='running' FOR UPDATE OF runs`, runID).Scan(&automationID, &householdID); err != nil {
			return fmt.Errorf("lock automation run: %w", err)
		}
		rows, err := executor.QueryContext(txctx, `SELECT actions.device_id,actions.action,actions.ordinal,devices.household_id,devices.state FROM automation_actions actions JOIN devices ON devices.id=actions.device_id WHERE actions.automation_id=$1 ORDER BY actions.ordinal`, automationID)
		if err != nil {
			return fmt.Errorf("load automation actions: %w", err)
		}
		type action struct {
			deviceID  int64
			action    string
			ordinal   int
			household int64
			state     model.DeviceState
		}
		var actions []action
		for rows.Next() {
			var item action
			if err := rows.Scan(&item.deviceID, &item.action, &item.ordinal, &item.household, &item.state); err != nil {
				_ = rows.Close()
				return fmt.Errorf("scan automation action: %w", err)
			}
			actions = append(actions, item)
		}
		if err := rows.Close(); err != nil {
			return fmt.Errorf("close automation actions: %w", err)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate automation actions: %w", err)
		}
		if len(actions) == 0 {
			return fmt.Errorf("%w: automation has no actions", model.ErrConflict)
		}
		for _, item := range actions {
			if item.household != householdID || item.state != model.DeviceEnabled {
				return fmt.Errorf("%w: automation device %d is not enabled in household %d", model.ErrConflict, item.deviceID, householdID)
			}
			if _, err := executor.ExecContext(txctx, `INSERT INTO outbox_messages(household_id,topic,payload,available_at) VALUES($1,'device.command',jsonb_build_object('automation_id',$2::bigint,'run_id',$3::bigint,'device_id',$4::bigint,'action',$5::text,'ordinal',$6::int),$7)`, householdID, automationID, runID, item.deviceID, item.action, item.ordinal, finishedAt); err != nil {
				return fmt.Errorf("enqueue automation action: %w", err)
			}
		}
		requestID := fmt.Sprintf("automation-run-%d", runID)
		if _, err := executor.ExecContext(txctx, `INSERT INTO audit_events(household_id,request_id,object_type,object_id,action,payload) VALUES($1,$2,'automation_run',$3,'dispatched',jsonb_build_object('action_count',$4::int))`, householdID, requestID, fmt.Sprint(runID), len(actions)); err != nil {
			return fmt.Errorf("audit automation run: %w", err)
		}
		result, err := executor.ExecContext(txctx, `UPDATE automation_runs SET state='succeeded',error_text=NULL,finished_at=$2 WHERE id=$1 AND state='running'`, runID, finishedAt)
		if err != nil {
			return fmt.Errorf("finish automation run: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("inspect automation run update: %w", err)
		}
		if affected != 1 {
			return model.ErrConflict
		}
		return nil
	})
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
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
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
