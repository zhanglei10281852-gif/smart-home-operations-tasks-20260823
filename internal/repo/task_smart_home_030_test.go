package repo_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/db"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/repo"
)

func TestSerializableCallbackIsNotReplayedAfterExternalSideEffect(t *testing.T) {
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
	r := repo.New(database)

	var notifications []string
	conflict := &pq.Error{Code: "40001", Message: "serialization failure after gateway notification"}
	err = r.WithSerializable(ctx, func(context.Context) error {
		notifications = append(notifications, "gateway-offline")
		return conflict
	})
	var postgres *pq.Error
	if !errors.As(err, &postgres) || postgres.Code != "40001" {
		t.Fatalf("serialization error was not returned intact: %v", err)
	}
	if len(notifications) != 1 {
		t.Fatalf("transaction wrapper replayed an arbitrary callback: notifications=%v", notifications)
	}

	validCalls := 0
	if err := r.WithSerializable(ctx, func(context.Context) error {
		validCalls++
		return nil
	}); err != nil {
		t.Fatalf("valid serializable callback failed: %v", err)
	}
	if validCalls != 1 {
		t.Fatalf("valid callback invocation count=%d", validCalls)
	}
}
