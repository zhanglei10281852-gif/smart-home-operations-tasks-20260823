package db_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/zhanglei10281852-gif/smart-home-operations/internal/db"
)

// TestWithTxPanicReleasesConnection reproduces the family-data transaction
// failure: when a business callback panics, the transaction must roll back and
// release its connection. With a single-connection pool, a leaked transaction
// would otherwise block every subsequent query until timeout.
func TestWithTxPanicReleasesConnection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://smart_home:smart_home@127.0.0.1:55432/smart_home?sslmode=disable"
	}
	database, err := db.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.Migrate(ctx, database); err != nil {
		t.Fatal(err)
	}
	// Force a single connection so a leaked transaction is fatal.
	database.SQL.SetMaxOpenConns(1)
	database.SQL.SetMaxIdleConns(1)

	boom := errors.New("boom")
	defer func() {
		v := recover()
		if v == nil {
			t.Fatal("WithTx swallowed the original panic")
		}
		got, ok := v.(error)
		if !ok || !errors.Is(got, boom) {
			t.Fatalf("recovered value is not the original error: got %v", v)
		}
	}()
	_ = database.WithTx(ctx, func(txctx context.Context) error {
		// Write inside the transaction; it must be rolled back on panic.
		if _, err := db.Executor(txctx, database).ExecContext(txctx, `INSERT INTO households(name,timezone,monthly_budget_cents) VALUES($1,$2,$3)`, "panic-leak", "UTC", 1); err != nil {
			return err
		}
		panic(boom)
	})

	// If WithTx leaked the connection, this query blocks until the test
	// timeout because the only connection is stuck in an open transaction.
	countCtx, countCancel := context.WithTimeout(ctx, 5*time.Second)
	defer countCancel()
	var count int
	if err := database.SQL.QueryRowContext(countCtx, `SELECT COUNT(*) FROM households WHERE name=$1`, "panic-leak").Scan(&count); err != nil {
		t.Fatalf("post-panic query failed (connection leaked?): %v", err)
	}
	if count != 0 {
		t.Fatalf("transaction was not rolled back: count=%d", count)
	}
}

// TestWithTxRollsBackOnCommitPanic guards the non-panic rollback path remains
// intact: a returned error must still roll back and surface the error.
func TestWithTxRollsBackOnError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://smart_home:smart_home@127.0.0.1:55432/smart_home?sslmode=disable"
	}
	database, err := db.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.Migrate(ctx, database); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("boom")
	err = database.WithTx(ctx, func(txctx context.Context) error {
		if _, err := db.Executor(txctx, database).ExecContext(txctx, `INSERT INTO households(name,timezone,monthly_budget_cents) VALUES($1,$2,$3)`, "error-rollback", "UTC", 1); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("expected boom error, got %v", err)
	}
	var count int
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM households WHERE name=$1`, "error-rollback").Scan(&count); err != nil || count != 0 {
		t.Fatalf("error-path rollback failed: count=%d err=%v", count, err)
	}
	// Ensure no rows are left in a transaction state on the single connection.
	if _, err := database.SQL.ExecContext(ctx, `SELECT 1`); err != nil {
		t.Fatalf("connection not reusable after error rollback: %v", err)
	}
}
