package worker

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetryStateProgression(t *testing.T) {
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	state := RetryState{Limit: 3}
	state = NextRetry(state, errors.New("temporary"), now, func(int) time.Duration { return time.Second })
	if state.Attempt != 1 || state.Permanent || !state.Next.Equal(now.Add(time.Second)) || state.LastError == "" {
		t.Fatalf("state=%+v", state)
	}
	state = NextRetry(state, errors.New("temporary"), now, func(int) time.Duration { return 2 * time.Second })
	state = NextRetry(state, errors.New("permanent"), now, func(int) time.Duration { return time.Second })
	if !state.Permanent || state.Attempt != 3 {
		t.Fatalf("state=%+v", state)
	}
	if err := state.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestHealthChecksAndGuardedRunner(t *testing.T) {
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	results := RunHealthChecks(context.Background(), map[string]HealthCheck{"ok": func(context.Context) error { return nil }, "bad": func(context.Context) error { return errors.New("down") }}, func() time.Time { return now })
	if len(results) != 2 {
		t.Fatalf("results=%+v", results)
	}
	if err := (GuardedRunner{Run: func(context.Context) error { return context.Canceled }}).Serve(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := (GuardedRunner{Run: func(context.Context) error { return errors.New("failed") }}).Serve(context.Background()); err == nil {
		t.Fatal("worker error swallowed")
	}
	if err := (GuardedRunner{Run: func(context.Context) error { panic("failed") }}).Serve(context.Background()); err == nil {
		t.Fatal("worker panic swallowed")
	}
	results = RunHealthChecks(context.Background(), map[string]HealthCheck{"missing": nil}, nil)
	if len(results) != 1 || results[0].Healthy || results[0].Error == "" {
		t.Fatalf("nil health check results=%+v", results)
	}
}
