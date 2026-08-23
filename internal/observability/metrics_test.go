package observability

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRegistryCountersAndExport(t *testing.T) {
	r := NewRegistry()
	c := r.Counter("requests_total", map[string]string{"route": "/healthz"})
	c.Add(2)
	c.Add(3)
	if c.Value() != 5 || r.Counter("requests_total", nil) != c {
		t.Fatal("counter was not stable")
	}
	snapshot := r.Snapshot(time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC))
	if len(snapshot) != 1 || snapshot[0].Name != "requests_total" || snapshot[0].Value != 5 {
		t.Fatalf("snapshot=%+v", snapshot)
	}
	exported, err := r.Export(context.Background(), time.Now())
	if err != nil || exported != "requests_total 5\n" {
		t.Fatalf("export=%q err=%v", exported, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.Export(ctx, time.Now()); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel err=%v", err)
	}
}
