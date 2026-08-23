package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

type DB struct{ SQL *sql.DB }

func Open(ctx context.Context, dsn string) (*DB, error) {
	database, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	database.SetMaxOpenConns(20)
	database.SetMaxIdleConns(8)
	database.SetConnMaxLifetime(30 * time.Minute)
	if err = database.PingContext(ctx); err != nil {
		database.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &DB{SQL: database}, nil
}
func (d *DB) Close() error { return d.SQL.Close() }
func (d *DB) WithTx(ctx context.Context, fn func(context.Context) error) error {
	tx, err := d.SQL.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	txctx := context.WithValue(ctx, txKey{}, tx)
	if err = fn(txctx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

type txKey struct{}

func Executor(ctx context.Context, database *DB) interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
} {
	if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return tx
	}
	return database.SQL
}
func Migrate(ctx context.Context, database *DB) error {
	if _, err := database.SQL.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return err
	}
	versions := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			versions = append(versions, entry.Name())
		}
	}
	sort.Strings(versions)
	for _, version := range versions {
		var applied bool
		if err := database.SQL.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, version).Scan(&applied); err != nil {
			return err
		}
		if applied {
			continue
		}
		content, readErr := fs.ReadFile(migrationFS, "migrations/"+version)
		if readErr != nil {
			return readErr
		}
		if err := database.WithTx(ctx, func(txctx context.Context) error {
			executor := Executor(txctx, database)
			if _, e := executor.ExecContext(txctx, string(content)); e != nil {
				return e
			}
			_, e := executor.ExecContext(txctx, `INSERT INTO schema_migrations(version) VALUES($1)`, version)
			return e
		}); err != nil {
			return fmt.Errorf("migration %s: %w", version, err)
		}
	}
	return nil
}

func MigrationVersions(ctx context.Context, database *DB) ([]string, error) {
	if database == nil || database.SQL == nil {
		return nil, fmt.Errorf("database is not configured")
	}
	rows, err := database.SQL.QueryContext(ctx, `SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var versions []string
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}
	return versions, rows.Err()
}
