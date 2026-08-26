package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type HealthReport struct {
	Reachable bool
	Latency   time.Duration
	Version   string
	At        time.Time
}

func (d *DB) Health(ctx context.Context, now func() time.Time) (HealthReport, error) {
	if d == nil || d.SQL == nil {
		return HealthReport{}, errors.New("database is not configured")
	}
	if now == nil {
		now = time.Now
	}
	started := time.Now()
	if err := d.SQL.PingContext(ctx); err != nil {
		return HealthReport{Latency: time.Since(started), At: now()}, err
	}
	var version string
	if err := d.SQL.QueryRowContext(ctx, `SELECT current_setting('server_version')`).Scan(&version); err != nil {
		return HealthReport{}, fmt.Errorf("read server version: %w", err)
	}
	return HealthReport{Reachable: true, Latency: time.Since(started), Version: version, At: now()}, nil
}

func (d *DB) WithReadOnlyTx(ctx context.Context, fn func(context.Context, *sql.Tx) error) error {
	if d == nil || d.SQL == nil || fn == nil {
		return errors.New("read-only transaction is not configured")
	}
	tx, err := d.SQL.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	txctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if err := fn(txctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}
