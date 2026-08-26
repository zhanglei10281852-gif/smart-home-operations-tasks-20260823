package db_test

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/db"
)

func TestMigrationFailureDoesNotMarkVersionBeforeSchemaCommit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://smart_home:smart_home@127.0.0.1:55432/smart_home?sslmode=disable"
	}

	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	const schema = "task_smart_home_028"
	if _, err := admin.ExecContext(ctx, `DROP SCHEMA IF EXISTS task_smart_home_028 CASCADE`); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.ExecContext(ctx, `CREATE SCHEMA task_smart_home_028`); err != nil {
		t.Fatal(err)
	}
	defer admin.ExecContext(context.Background(), `DROP SCHEMA IF EXISTS task_smart_home_028 CASCADE`)

	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	database, err := db.Open(ctx, parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	if err := db.Migrate(ctx, database); err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `ALTER TABLE devices DROP CONSTRAINT devices_kind_check`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version='002_state_constraints.sql'`); err != nil {
		t.Fatal(err)
	}
	var householdID int64
	if err := database.SQL.QueryRowContext(ctx, `INSERT INTO households(name,timezone,monthly_budget_cents) VALUES('migration-conflict','UTC',0) RETURNING id`).Scan(&householdID); err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `INSERT INTO devices(household_id,external_id,kind,state,firmware) VALUES($1,'legacy-kind','unsupported','pending','0.9')`, householdID); err != nil {
		t.Fatal(err)
	}

	if err := db.Migrate(ctx, database); err == nil {
		t.Fatal("migration with conflicting historical data unexpectedly succeeded")
	}
	var markedAfterFailure bool
	if err := database.SQL.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version='002_state_constraints.sql')`).Scan(&markedAfterFailure); err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `DELETE FROM devices WHERE external_id='legacy-kind'`); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx, database); err != nil {
		t.Fatalf("migration did not recover after historical data was corrected: %v", err)
	}

	var constraintAfterRestart bool
	if err := database.SQL.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='devices_kind_check' AND conrelid='devices'::regclass)`).Scan(&constraintAfterRestart); err != nil {
		t.Fatal(err)
	}
	_, invalidInsertErr := database.SQL.ExecContext(ctx, `INSERT INTO devices(household_id,external_id,kind,state,firmware) VALUES($1,'new-invalid-kind','unsupported','pending','1.0')`, householdID)
	if invalidInsertErr == nil {
		_, _ = database.SQL.ExecContext(ctx, `DELETE FROM devices WHERE external_id='new-invalid-kind'`)
	}
	if markedAfterFailure || !constraintAfterRestart || invalidInsertErr == nil {
		t.Fatalf("migration metadata/schema diverged: marked_after_failure=%v constraint_after_restart=%v invalid_insert_err=%v", markedAfterFailure, constraintAfterRestart, invalidInsertErr)
	}
}
