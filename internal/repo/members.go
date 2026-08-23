package repo

import (
	"context"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"time"
)

func (r *Repository) SetMemberActive(ctx context.Context, id int64, active bool) error {
	res, err := r.executor(ctx).ExecContext(ctx, `UPDATE members SET active=$2 WHERE id=$1`, id, active)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return model.ErrNotFound
	}
	return nil
}
func (r *Repository) Members(ctx context.Context, household int64) ([]model.Member, error) {
	rows, err := r.executor(ctx).QueryContext(ctx, `SELECT id,household_id,email,role,active,created_at FROM members WHERE household_id=$1 ORDER BY id`, household)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Member{}
	for rows.Next() {
		var m model.Member
		if err := rows.Scan(&m.ID, &m.HouseholdID, &m.Email, &m.Role, &m.Active, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
func (r *Repository) SessionCount(ctx context.Context, member int64, now time.Time) (int, error) {
	var n int
	err := r.executor(ctx).QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE member_id=$1 AND revoked_at IS NULL AND expires_at > $2`, member, now).Scan(&n)
	return n, err
}
