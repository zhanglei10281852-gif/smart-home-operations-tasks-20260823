package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"time"
)

type OutboxFilter struct {
	Topic  string
	Before time.Time
	Limit  int
}

func (r *Repository) AcknowledgeOutbox(ctx context.Context, id int64) error {
	if id <= 0 {
		return model.ErrInvalid
	}
	return r.MarkOutbox(ctx, id, true, time.Time{})
}

func (r *Repository) ListOutbox(ctx context.Context, f OutboxFilter) ([]model.OutboxMessage, error) {
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 100
	}
	query := `SELECT id,household_id,topic,payload,attempts,available_at,locked_at,delivered_at,failed_at,failure_reason FROM outbox_messages WHERE delivered_at IS NULL AND failed_at IS NULL`
	args := []any{}
	if f.Topic != "" {
		query += ` AND topic=$1`
		args = append(args, f.Topic)
	}
	if !f.Before.IsZero() {
		query += ` AND available_at <= $` + itoa(len(args)+1)
		args = append(args, f.Before)
	}
	query += ` ORDER BY available_at,id LIMIT $` + itoa(len(args)+1)
	args = append(args, f.Limit)
	rows, err := r.executor(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.OutboxMessage{}
	for rows.Next() {
		var m model.OutboxMessage
		if err := rows.Scan(&m.ID, &m.HouseholdID, &m.Topic, &m.Payload, &m.Attempts, &m.AvailableAt, &m.LockedAt, &m.DeliveredAt, &m.FailedAt, &m.FailureReason); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
func (r *Repository) RescheduleOutbox(ctx context.Context, id int64, attempt int, next time.Time, err error) error {
	payload := map[string]any{"attempt": attempt, "error": ""}
	if err != nil {
		payload["error"] = err.Error()
	}
	data, _ := json.Marshal(payload)
	result, e := r.executor(ctx).ExecContext(ctx, `UPDATE outbox_messages SET attempts=$2,available_at=$3,locked_at=NULL,payload=payload || $4::jsonb WHERE id=$1 AND delivered_at IS NULL AND failed_at IS NULL`, id, attempt, next, data)
	if e != nil {
		return e
	}
	affected, e := result.RowsAffected()
	if e != nil {
		return e
	}
	if affected != 1 {
		return model.ErrConflict
	}
	return nil
}
func (r *Repository) MarkOutboxFailed(ctx context.Context, id int64, reason string) error {
	result, err := r.executor(ctx).ExecContext(ctx, `UPDATE outbox_messages SET locked_at=NULL,failed_at=now(),failure_reason=$2 WHERE id=$1 AND delivered_at IS NULL AND failed_at IS NULL`, id, reason)
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
func itoa(v int) string {
	switch v {
	case 1:
		return "1"
	case 2:
		return "2"
	case 3:
		return "3"
	case 4:
		return "4"
	case 5:
		return "5"
	case 6:
		return "6"
	case 7:
		return "7"
	case 8:
		return "8"
	default:
		return "9"
	}
}

var _ sql.Result
