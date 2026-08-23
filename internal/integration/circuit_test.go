package integration

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCircuitOpensAndRecovers(t *testing.T) {
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	c := NewCircuit(2, time.Minute)
	failure := errors.New("gateway down")
	if err := c.Call(context.Background(), now, func(context.Context) error { return failure }); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if err := c.Call(context.Background(), now, func(context.Context) error { return failure }); !errors.Is(err, failure) || c.State(now) != CircuitOpen {
		t.Fatalf("state=%s err=%v", c.State(now), err)
	}
	if err := c.Call(context.Background(), now.Add(time.Second), func(context.Context) error { return nil }); err == nil {
		t.Fatal("open circuit allowed call")
	}
	if err := c.Call(context.Background(), now.Add(time.Minute), func(context.Context) error { return nil }); err != nil || c.State(now.Add(time.Minute)) != CircuitClosed {
		t.Fatalf("recovery state=%s err=%v", c.State(now.Add(time.Minute)), err)
	}
}

func TestCircuitCancellationDoesNotCountAsFailure(t *testing.T) {
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	c := NewCircuit(1, time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := c.Call(ctx, now, func(context.Context) error { t.Fatal("cancelled callback called"); return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
	if c.State(now) != CircuitClosed {
		t.Fatalf("cancelled call opened circuit: %s", c.State(now))
	}
}
