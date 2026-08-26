package repo_test

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/db"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/repo"
)

func TestSerializableCallbackPanicReleasesTransactionConnection(t *testing.T) {
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
	if _, err := admin.ExecContext(ctx, `DROP SCHEMA IF EXISTS task_smart_home_029 CASCADE`); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.ExecContext(ctx, `CREATE SCHEMA task_smart_home_029; CREATE TABLE task_smart_home_029.tx_probe(id BIGINT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	defer admin.ExecContext(context.Background(), `DROP SCHEMA IF EXISTS task_smart_home_029 CASCADE`)

	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	query.Set("search_path", "task_smart_home_029")
	parsed.RawQuery = query.Encode()
	database, err := db.Open(ctx, parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	database.SQL.SetMaxOpenConns(1)
	database.SQL.SetMaxIdleConns(1)
	defer database.Close()
	r := repo.New(database)

	const panicValue = "serializable callback failed"
	var recovered any
	var callbackErr error
	var backendPID int
	func() {
		defer func() { recovered = recover() }()
		callbackErr = r.WithSerializable(ctx, func(txctx context.Context) error {
			if _, err := r.ExecInTransaction(txctx, `INSERT INTO tx_probe(id) VALUES(1)`); err != nil {
				return err
			}
			if err := db.Executor(txctx, database).QueryRowContext(txctx, `SELECT pg_backend_pid()`).Scan(&backendPID); err != nil {
				return err
			}
			panic(panicValue)
		})
	}()
	if callbackErr != nil {
		t.Fatalf("transaction callback setup failed before panic: %v", callbackErr)
	}
	if recovered != panicValue {
		t.Fatalf("transaction wrapper changed the callback panic: recovered=%v", recovered)
	}

	followCtx, cancelFollow := context.WithTimeout(context.Background(), 350*time.Millisecond)
	defer cancelFollow()
	var one int
	followErr := database.SQL.QueryRowContext(followCtx, `SELECT 1`).Scan(&one)
	if backendPID != 0 {
		_, _ = admin.ExecContext(context.Background(), `SELECT pg_terminate_backend($1)`, backendPID)
	}
	var leakedRows int
	if err := admin.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_smart_home_029.tx_probe`).Scan(&leakedRows); err != nil {
		t.Fatal(err)
	}
	if followErr != nil || one != 1 || leakedRows != 0 {
		t.Fatalf("panic leaked transaction ownership: follow_err=%v follow_value=%d uncommitted_rows=%d", followErr, one, leakedRows)
	}
}
