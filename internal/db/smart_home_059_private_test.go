package db

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDatabaseHealthReleasesItsOnlyConnection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	database, err := Open(ctx, "postgres://smart_home:smart_home@127.0.0.1:55432/smart_home?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.SQL.SetMaxOpenConns(1)
	database.SQL.SetMaxIdleConns(1)

	report, err := database.Health(ctx, time.Now)
	if err != nil || !report.Reachable || report.Version == "" {
		t.Fatalf("health report=%+v err=%v", report, err)
	}
	followupCtx, cancelFollowup := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancelFollowup()
	err = database.SQL.PingContext(followupCtx)
	if errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("health query kept the only database connection after returning")
	}
	if err != nil {
		t.Fatalf("follow-up database use failed: %v", err)
	}
}
