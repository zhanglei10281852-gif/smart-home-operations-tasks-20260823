package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zhanglei10281852-gif/smart-home-operations/internal/db"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
)

type TransactionalDeviceUpdate struct {
	DeviceID int64
	From, To model.DeviceState
	Version  int64
	Audit    model.AuditEvent
	Outbox   model.OutboxMessage
}

func (r *Repository) TransitionWithAudit(ctx context.Context, u TransactionalDeviceUpdate) error {
	return r.DB.WithTx(ctx, func(txctx context.Context) error {
		ex := r.executor(txctx)
		res, err := ex.ExecContext(txctx, `UPDATE devices SET state=$2,version=version+1 WHERE id=$1 AND state=$3 AND version=$4`, u.DeviceID, u.To, u.From, u.Version)
		if err != nil {
			return fmt.Errorf("update device state: %w", err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("inspect device update: %w", err)
		}
		if n != 1 {
			return model.ErrConflict
		}
		if err := r.AddAudit(txctx, u.Audit); err != nil {
			return fmt.Errorf("write audit: %w", err)
		}
		if u.Outbox.Topic != "" {
			if err := r.AddOutbox(txctx, u.Outbox); err != nil {
				return fmt.Errorf("write outbox: %w", err)
			}
		}
		return nil
	})
}
func (r *Repository) WithSerializable(ctx context.Context, fn func(context.Context) error) error {
	return r.DB.WithTx(ctx, fn)
}

func (r *Repository) ExecInTransaction(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return db.Executor(ctx, r.DB).ExecContext(ctx, query, args...)
}
func (r *Repository) AuditAndMessage(ctx context.Context, a model.AuditEvent, m model.OutboxMessage) error {
	return r.DB.WithTx(ctx, func(txctx context.Context) error {
		if err := r.AddAudit(txctx, a); err != nil {
			return err
		}
		if m.Topic == "" {
			return nil
		}
		return r.AddOutbox(txctx, m)
	})
}
func RollbackProof(ctx context.Context, r *Repository, household int64) error {
	return r.DB.WithTx(ctx, func(txctx context.Context) error {
		if _, err := r.CreateHousehold(txctx, "rollback-proof", "UTC", 100); err != nil {
			return err
		}
		return model.ErrConflict
	})
}
func (r *Repository) VacuumSessions(ctx context.Context, now time.Time) (int64, error) {
	return r.DeleteExpiredSessions(ctx, now)
}
