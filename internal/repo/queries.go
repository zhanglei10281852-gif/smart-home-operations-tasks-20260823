package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
)

type DeviceFilter struct {
	HouseholdID int64
	States      []model.DeviceState
	Kinds       []model.DeviceKind
	Query       string
	Limit       int
	Offset      int
}

func (f DeviceFilter) Normalize() (DeviceFilter, error) {
	if f.HouseholdID <= 0 || f.Offset < 0 || f.Limit < 0 || f.Limit > 200 {
		return f, model.ErrInvalid
	}
	if f.Limit == 0 {
		f.Limit = 50
	}
	f.Query = strings.TrimSpace(f.Query)
	return f, nil
}

func (r *Repository) SearchDevices(ctx context.Context, filter DeviceFilter) ([]model.Device, error) {
	filter, err := filter.Normalize()
	if err != nil {
		return nil, err
	}
	where := []string{"household_id=$1"}
	args := []any{filter.HouseholdID}
	arg := 2
	if filter.Query != "" {
		where = append(where, fmt.Sprintf("(external_id ILIKE $%d OR firmware ILIKE $%d)", arg, arg))
		args = append(args, "%"+filter.Query+"%")
		arg++
	}
	if len(filter.States) > 0 {
		marks := make([]string, len(filter.States))
		for i, state := range filter.States {
			marks[i] = fmt.Sprintf("$%d", arg+i)
			args = append(args, state)
		}
		where = append(where, "state IN ("+strings.Join(marks, ",")+")")
		arg += len(filter.States)
	}
	if len(filter.Kinds) > 0 {
		marks := make([]string, len(filter.Kinds))
		for i, kind := range filter.Kinds {
			marks[i] = fmt.Sprintf("$%d", arg+i)
			args = append(args, kind)
		}
		where = append(where, "kind IN ("+strings.Join(marks, ",")+")")
		arg += len(filter.Kinds)
	}
	args = append(args, filter.Limit, filter.Offset)
	query := `SELECT id,household_id,external_id,kind,state,firmware,version,last_seen_at,created_at FROM devices WHERE ` + strings.Join(where, " AND ") + fmt.Sprintf(" ORDER BY created_at DESC,id DESC LIMIT $%d OFFSET $%d", arg, arg+1)
	rows, err := r.executor(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]model.Device, 0, filter.Limit)
	for rows.Next() {
		var d model.Device
		if err := rows.Scan(&d.ID, &d.HouseholdID, &d.ExternalID, &d.Kind, &d.State, &d.Firmware, &d.Version, &d.LastSeenAt, &d.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

func (r *Repository) CountDevices(ctx context.Context, household int64, state model.DeviceState) (int64, error) {
	if household <= 0 {
		return 0, model.ErrInvalid
	}
	var count int64
	query := `SELECT COUNT(*) FROM devices WHERE household_id=$1`
	args := []any{household}
	if state != "" {
		query += ` AND state=$2`
		args = append(args, state)
	}
	if err := r.executor(ctx).QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *Repository) AppendAuditAndOutbox(ctx context.Context, event model.AuditEvent, outbox model.OutboxMessage) error {
	if event.HouseholdID <= 0 || event.RequestID == "" || event.ObjectID == "" || event.Action == "" {
		return model.ErrInvalid
	}
	return r.DB.WithTx(ctx, func(txctx context.Context) error {
		if err := r.AddAudit(txctx, event); err != nil {
			return fmt.Errorf("audit: %w", err)
		}
		if outbox.Topic == "" {
			return nil
		}
		if err := r.AddOutbox(txctx, outbox); err != nil {
			return fmt.Errorf("outbox: %w", err)
		}
		return nil
	})
}

func (r *Repository) ReplayableOutbox(ctx context.Context, before time.Time, limit int) ([]model.OutboxMessage, error) {
	if before.IsZero() || limit <= 0 || limit > 500 {
		return nil, model.ErrInvalid
	}
	rows, err := r.executor(ctx).QueryContext(ctx, `SELECT id,household_id,topic,payload,attempts,available_at,locked_at,delivered_at FROM outbox_messages WHERE delivered_at IS NULL AND available_at <= $1 ORDER BY available_at,id LIMIT $2`, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []model.OutboxMessage
	for rows.Next() {
		var m model.OutboxMessage
		if err := rows.Scan(&m.ID, &m.HouseholdID, &m.Topic, &m.Payload, &m.Attempts, &m.AvailableAt, &m.LockedAt, &m.DeliveredAt); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if result == nil {
		return []model.OutboxMessage{}, nil
	}
	return result, nil
}

func (r *Repository) DeleteExpiredSessions(ctx context.Context, now time.Time) (int64, error) {
	if now.IsZero() {
		return 0, model.ErrInvalid
	}
	result, err := r.executor(ctx).ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= $1 OR revoked_at IS NOT NULL`, now)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func scanNullableTime(row *sql.Row, dst *time.Time) error {
	var value sql.NullTime
	if err := row.Scan(&value); err != nil {
		return err
	}
	if !value.Valid {
		return errors.New("expected non-null timestamp")
	}
	*dst = value.Time
	return nil
}
