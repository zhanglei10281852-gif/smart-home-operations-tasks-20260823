package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/db"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"time"
)

type Repository struct{ DB *db.DB }

func New(database *db.DB) *Repository { return &Repository{DB: database} }
func (r *Repository) executor(ctx context.Context) interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
} {
	return db.Executor(ctx, r.DB)
}

func (r *Repository) CreateHousehold(ctx context.Context, name, timezone string, budget int64) (model.Household, error) {
	if name == "" || timezone == "" || budget < 0 {
		return model.Household{}, model.ErrInvalid
	}
	var h model.Household
	err := r.executor(ctx).QueryRowContext(ctx, `INSERT INTO households(name,timezone,monthly_budget_cents) VALUES($1,$2,$3) RETURNING id,name,timezone,monthly_budget_cents,created_at`, name, timezone, budget).Scan(&h.ID, &h.Name, &h.Timezone, &h.MonthlyBudgetCents, &h.CreatedAt)
	return h, classifyWriteError(err)
}
func (r *Repository) CreateHouseholdWithOwner(ctx context.Context, name, timezone string, budget int64, email, hash string) (model.Household, model.Member, error) {
	var household model.Household
	var owner model.Member
	err := r.DB.WithTx(ctx, func(txctx context.Context) error {
		var err error
		household, err = r.CreateHousehold(txctx, name, timezone, budget)
		if err != nil {
			return fmt.Errorf("create household: %w", err)
		}
		owner, err = r.AddMember(txctx, household.ID, email, hash, model.RoleOwner)
		if err != nil {
			return fmt.Errorf("create owner: %w", err)
		}
		return nil
	})
	return household, owner, err
}
func (r *Repository) AddMember(ctx context.Context, householdID int64, email, hash string, role model.Role) (model.Member, error) {
	var m model.Member
	err := r.executor(ctx).QueryRowContext(ctx, `INSERT INTO members(household_id,email,password_hash,role) VALUES($1,$2,$3,$4) RETURNING id,household_id,email,role,active,created_at`, householdID, email, hash, role).Scan(&m.ID, &m.HouseholdID, &m.Email, &m.Role, &m.Active, &m.CreatedAt)
	return m, classifyWriteError(err)
}
func (r *Repository) FindMember(ctx context.Context, householdID int64, email string) (model.Member, string, error) {
	var m model.Member
	var hash string
	err := r.executor(ctx).QueryRowContext(ctx, `SELECT id,household_id,email,password_hash,role,active,created_at FROM members WHERE household_id=$1 AND email=$2`, householdID, email).Scan(&m.ID, &m.HouseholdID, &m.Email, &hash, &m.Role, &m.Active, &m.CreatedAt)
	return m, hash, classifyReadError(err)
}
func (r *Repository) MemberByID(ctx context.Context, id int64) (model.Member, error) {
	var m model.Member
	err := r.executor(ctx).QueryRowContext(ctx, `SELECT id,household_id,email,role,active,created_at FROM members WHERE id=$1`, id).Scan(&m.ID, &m.HouseholdID, &m.Email, &m.Role, &m.Active, &m.CreatedAt)
	return m, classifyReadError(err)
}
func (r *Repository) CreateSession(ctx context.Context, s model.Session) error {
	_, err := r.executor(ctx).ExecContext(ctx, `INSERT INTO sessions(id,member_id,expires_at) VALUES($1,$2,$3)`, s.ID, s.MemberID, s.ExpiresAt)
	return classifyWriteError(err)
}
func (r *Repository) GetSession(ctx context.Context, id string) (model.Session, error) {
	var s model.Session
	err := r.executor(ctx).QueryRowContext(ctx, `SELECT id,member_id,expires_at,revoked_at,created_at FROM sessions WHERE id=$1`, id).Scan(&s.ID, &s.MemberID, &s.ExpiresAt, &s.RevokedAt, &s.CreatedAt)
	return s, classifyReadError(err)
}
func (r *Repository) RevokeSession(ctx context.Context, id string, now time.Time) error {
	res, err := r.executor(ctx).ExecContext(ctx, `UPDATE sessions SET revoked_at=$2 WHERE id=$1 AND revoked_at IS NULL`, id, now)
	if err != nil {
		return classifyWriteError(err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return model.ErrNotFound
	}
	return nil
}

func (r *Repository) CreateDevice(ctx context.Context, d model.Device, capabilities []string) (model.Device, error) {
	var out model.Device
	err := r.DB.WithTx(ctx, func(txctx context.Context) error {
		ex := r.executor(txctx)
		if e := ex.QueryRowContext(txctx, `INSERT INTO devices(household_id,external_id,kind,state,firmware) VALUES($1,$2,$3,$4,$5) RETURNING id,household_id,external_id,kind,state,firmware,version,created_at`, d.HouseholdID, d.ExternalID, d.Kind, model.DevicePending, d.Firmware).Scan(&out.ID, &out.HouseholdID, &out.ExternalID, &out.Kind, &out.State, &out.Firmware, &out.Version, &out.CreatedAt); e != nil {
			return classifyWriteError(e)
		}
		if e := r.insertDeviceCapabilities(txctx, out.ID, capabilities); e != nil {
			return e
		}
		return nil
	})
	if err != nil {
		return model.Device{}, err
	}
	return out, nil
}
func (r *Repository) GetDevice(ctx context.Context, id int64) (model.Device, error) {
	var d model.Device
	err := r.executor(ctx).QueryRowContext(ctx, `SELECT id,household_id,external_id,kind,state,firmware,version,last_seen_at,created_at FROM devices WHERE id=$1`, id).Scan(&d.ID, &d.HouseholdID, &d.ExternalID, &d.Kind, &d.State, &d.Firmware, &d.Version, &d.LastSeenAt, &d.CreatedAt)
	return d, classifyReadError(err)
}
func (r *Repository) TransitionDevice(ctx context.Context, id int64, from, to model.DeviceState, expectedVersion int64) error {
	res, err := r.executor(ctx).ExecContext(ctx, `UPDATE devices SET state=$2,version=version+1 WHERE id=$1 AND state=$3 AND version=$4`, id, to, from, expectedVersion)
	if err != nil {
		return classifyWriteError(err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return model.ErrConflict
	}
	return nil
}
func (r *Repository) TouchDevice(ctx context.Context, id int64, when time.Time) error {
	res, err := r.executor(ctx).ExecContext(ctx, `UPDATE devices SET last_seen_at=$2 WHERE id=$1`, id, when)
	if err != nil {
		return classifyWriteError(err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return model.ErrNotFound
	}
	return nil
}
func (r *Repository) AddCapability(ctx context.Context, id int64, capability string) error {
	if capability == "" {
		return model.ErrInvalid
	}
	_, err := r.executor(ctx).ExecContext(ctx, `INSERT INTO device_capabilities(device_id,capability) VALUES($1,$2) ON CONFLICT DO NOTHING`, id, capability)
	return classifyWriteError(err)
}

func (r *Repository) InsertTelemetry(ctx context.Context, t model.Telemetry) error {
	result, err := r.executor(ctx).ExecContext(ctx, `INSERT INTO telemetry(device_id,sequence,power_watts,temperature_c,measured_at) SELECT id,$2,$3,$4,$5 FROM devices WHERE id=$1 AND state='enabled'`, t.DeviceID, t.Sequence, t.PowerWatts, t.TemperatureC, t.MeasuredAt)
	if err != nil {
		return classifyWriteError(err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return fmt.Errorf("%w: telemetry device is not enabled", model.ErrConflict)
	}
	return nil
}
func (r *Repository) TelemetryWindow(ctx context.Context, deviceID int64, start, end time.Time) ([]model.Telemetry, error) {
	rows, err := r.executor(ctx).QueryContext(ctx, `SELECT id,device_id,sequence,power_watts,temperature_c,measured_at,received_at FROM telemetry WHERE device_id=$1 AND measured_at >= $2 AND measured_at < $3 ORDER BY measured_at,sequence`, deviceID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Telemetry{}
	for rows.Next() {
		var t model.Telemetry
		if err := rows.Scan(&t.ID, &t.DeviceID, &t.Sequence, &t.PowerWatts, &t.TemperatureC, &t.MeasuredAt, &t.ReceivedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *Repository) CreatePlan(ctx context.Context, p model.EnergyPlan, devices []model.PlanDevice) (model.EnergyPlan, error) {
	var out model.EnergyPlan
	err := r.DB.WithTx(ctx, func(txctx context.Context) error {
		ex := r.executor(txctx)
		if e := ex.QueryRowContext(txctx, `INSERT INTO energy_plans(household_id,name,state,budget_cents,starts_at,ends_at) VALUES($1,$2,$3,$4,$5,$6) RETURNING id,household_id,name,state,budget_cents,starts_at,ends_at,created_at`, p.HouseholdID, p.Name, model.PlanDraft, p.BudgetCents, p.StartsAt, p.EndsAt).Scan(&out.ID, &out.HouseholdID, &out.Name, &out.State, &out.BudgetCents, &out.StartsAt, &out.EndsAt, &out.CreatedAt); e != nil {
			return classifyWriteError(e)
		}
		for _, d := range devices {
			result, e := ex.ExecContext(txctx, `INSERT INTO plan_devices(plan_id,device_id,target_watts) SELECT $1,id,$3 FROM devices WHERE id=$2 AND household_id=$4 AND state='enabled'`, out.ID, d.DeviceID, d.TargetWatts, p.HouseholdID)
			if e != nil {
				return classifyWriteError(e)
			}
			affected, e := result.RowsAffected()
			if e != nil {
				return e
			}
			if affected != 1 {
				return fmt.Errorf("%w: plan device %d is not enabled in household %d", model.ErrConflict, d.DeviceID, p.HouseholdID)
			}
		}
		return nil
	})
	return out, err
}
func (r *Repository) PlanDevices(ctx context.Context, planID int64) ([]model.PlanDevice, error) {
	rows, err := r.executor(ctx).QueryContext(ctx, `SELECT plan_id,device_id,target_watts FROM plan_devices WHERE plan_id=$1 ORDER BY device_id`, planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.PlanDevice{}
	for rows.Next() {
		var d model.PlanDevice
		if err := rows.Scan(&d.PlanID, &d.DeviceID, &d.TargetWatts); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
func (r *Repository) SetPlanState(ctx context.Context, id int64, from, to model.PlanState) error {
	res, err := r.executor(ctx).ExecContext(ctx, `UPDATE energy_plans SET state=$2 WHERE id=$1 AND state=$3 AND ($2::text <> 'running' OR NOT EXISTS (SELECT 1 FROM plan_devices links JOIN devices ON devices.id=links.device_id WHERE links.plan_id=$1 AND (devices.state <> 'enabled' OR devices.household_id <> energy_plans.household_id)))`, id, to, from)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return model.ErrConflict
	}
	return nil
}

func (r *Repository) CreateAutomation(ctx context.Context, a model.Automation, actions []model.AutomationAction) (model.Automation, error) {
	var out model.Automation
	err := r.DB.WithTx(ctx, func(txctx context.Context) error {
		ex := r.executor(txctx)
		if e := ex.QueryRowContext(txctx, `INSERT INTO automations(household_id,name,state,trigger_kind) VALUES($1,$2,$3,$4) RETURNING id,household_id,name,state,trigger_kind,created_at`, a.HouseholdID, a.Name, model.AutomationDraft, a.TriggerKind).Scan(&out.ID, &out.HouseholdID, &out.Name, &out.State, &out.TriggerKind, &out.CreatedAt); e != nil {
			return classifyWriteError(e)
		}
		for _, action := range actions {
			result, e := ex.ExecContext(txctx, `INSERT INTO automation_actions(automation_id,device_id,action,ordinal) SELECT $1,id,$3,$4 FROM devices WHERE id=$2 AND household_id=$5 AND state='enabled'`, out.ID, action.DeviceID, action.Action, action.Ordinal, a.HouseholdID)
			if e != nil {
				return classifyWriteError(e)
			}
			affected, e := result.RowsAffected()
			if e != nil {
				return e
			}
			if affected != 1 {
				return fmt.Errorf("%w: automation device %d is not enabled in household %d", model.ErrConflict, action.DeviceID, a.HouseholdID)
			}
		}
		return nil
	})
	return out, err
}
func (r *Repository) QueueRun(ctx context.Context, automationID int64, key string) (model.AutomationRun, error) {
	var run model.AutomationRun
	err := r.executor(ctx).QueryRowContext(ctx, `INSERT INTO automation_runs(automation_id,idempotency_key,state) SELECT id,$2,$3 FROM automations WHERE id=$1 AND state='active' ON CONFLICT(automation_id,idempotency_key) DO UPDATE SET idempotency_key=EXCLUDED.idempotency_key RETURNING id,automation_id,idempotency_key,state`, automationID, key, model.RunQueued).Scan(&run.ID, &run.AutomationID, &run.IdempotencyKey, &run.State)
	if errors.Is(err, sql.ErrNoRows) {
		return model.AutomationRun{}, model.ErrConflict
	}
	return run, classifyWriteError(err)
}
func (r *Repository) ClaimRun(ctx context.Context) (model.AutomationRun, error) {
	var run model.AutomationRun
	err := r.executor(ctx).QueryRowContext(ctx, `WITH candidate AS (SELECT id FROM automation_runs WHERE state='queued' ORDER BY id FOR UPDATE SKIP LOCKED LIMIT 1) UPDATE automation_runs SET state='running',started_at=now() WHERE id=(SELECT id FROM candidate) RETURNING id,automation_id,idempotency_key,state,started_at`).Scan(&run.ID, &run.AutomationID, &run.IdempotencyKey, &run.State, &run.StartedAt)
	return run, err
}
func (r *Repository) FinishRun(ctx context.Context, id int64, state model.RunState, cause string, now time.Time) error {
	result, err := r.executor(ctx).ExecContext(ctx, `UPDATE automation_runs SET state=$2,error_text=$3,finished_at=$4 WHERE id=$1 AND state='running'`, id, state, cause, now)
	if err != nil {
		return classifyWriteError(err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return model.ErrConflict
	}
	return nil
}

func (r *Repository) RequeueRun(ctx context.Context, id int64) error {
	result, err := r.executor(ctx).ExecContext(ctx, `UPDATE automation_runs SET state='queued',started_at=NULL,error_text=NULL,finished_at=NULL WHERE id=$1 AND state='running'`, id)
	if err != nil {
		return classifyWriteError(err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return model.ErrConflict
	}
	return nil
}

func (r *Repository) AddAudit(ctx context.Context, e model.AuditEvent) error {
	_, err := r.executor(ctx).ExecContext(ctx, `INSERT INTO audit_events(household_id,actor_member_id,request_id,object_type,object_id,action,payload) VALUES($1,$2,$3,$4,$5,$6,$7)`, e.HouseholdID, e.ActorMemberID, e.RequestID, e.ObjectType, e.ObjectID, e.Action, e.Payload)
	return classifyWriteError(err)
}
func (r *Repository) AddOutbox(ctx context.Context, m model.OutboxMessage) error {
	_, err := r.executor(ctx).ExecContext(ctx, `INSERT INTO outbox_messages(household_id,topic,payload,available_at) VALUES($1,$2,$3,$4)`, m.HouseholdID, m.Topic, m.Payload, m.AvailableAt)
	return classifyWriteError(err)
}
func (r *Repository) ClaimOutbox(ctx context.Context) (model.OutboxMessage, error) {
	var m model.OutboxMessage
	err := r.executor(ctx).QueryRowContext(ctx, `WITH candidate AS (SELECT id FROM outbox_messages WHERE delivered_at IS NULL AND failed_at IS NULL AND available_at <= now() AND (locked_at IS NULL OR locked_at < now()-interval '5 minutes') ORDER BY id FOR UPDATE SKIP LOCKED LIMIT 1) UPDATE outbox_messages SET locked_at=now(),attempts=attempts+1 WHERE id=(SELECT id FROM candidate) RETURNING id,household_id,topic,payload,attempts,available_at,locked_at,delivered_at,failed_at,failure_reason`).Scan(&m.ID, &m.HouseholdID, &m.Topic, &m.Payload, &m.Attempts, &m.AvailableAt, &m.LockedAt, &m.DeliveredAt, &m.FailedAt, &m.FailureReason)
	return m, err
}
func (r *Repository) MarkOutbox(ctx context.Context, id int64, delivered bool, next time.Time) error {
	var result sql.Result
	var err error
	if delivered {
		result, err = r.executor(ctx).ExecContext(ctx, `UPDATE outbox_messages SET delivered_at=now(),locked_at=NULL WHERE id=$1 AND delivered_at IS NULL AND failed_at IS NULL`, id)
	} else {
		result, err = r.executor(ctx).ExecContext(ctx, `UPDATE outbox_messages SET locked_at=NULL,available_at=$2 WHERE id=$1 AND delivered_at IS NULL AND failed_at IS NULL`, id, next)
	}
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return model.ErrConflict
	}
	return nil
}
func Wrap(op string, err error) error { return fmt.Errorf("%s: %w", op, err) }
