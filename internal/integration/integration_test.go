package integration

import (
	"context"
	"errors"
	"testing"
	"time"
)

type gateway struct {
	calls int
	err   error
}

func (g *gateway) Command(context.Context, string, string, map[string]any) error {
	g.calls++
	return g.err
}
func (g *gateway) Health(context.Context, string) error { return g.err }
func TestDispatcherIdempotency(t *testing.T) {
	g := &gateway{}
	d := NewDispatcher(g)
	first := d.Dispatch(context.Background(), "key-1", "on", nil)
	second := d.Dispatch(context.Background(), "key-1", "on", nil)
	if !first.Accepted || !second.Accepted || g.calls != 1 {
		t.Fatalf("first=%+v second=%+v calls=%d", first, second, g.calls)
	}
	d.Forget("key-1")
	d.Dispatch(context.Background(), "key-1", "on", nil)
	if g.calls != 2 {
		t.Fatalf("calls after forget=%d", g.calls)
	}
}
func TestDispatcherTimeout(t *testing.T) {
	g := &gateway{err: context.DeadlineExceeded}
	d := NewDispatcher(g)
	d.Timeout = time.Millisecond
	r := d.Dispatch(context.Background(), "key", "off", nil)
	if r.Accepted || r.Error == "" {
		t.Fatalf("result=%+v", r)
	}
}
func TestGatewayHelpers(t *testing.T) {
	if _, err := EncodeCommand("id", "on", map[string]any{"x": 1}); err != nil {
		t.Fatal(err)
	}
	if !IsTransient(context.Canceled) || IsTransient(errors.New("other")) {
		t.Fatal("transient classifier incorrect")
	}
	g := &gateway{}
	snap := CheckGateway(context.Background(), g, "device")
	if !snap.Healthy {
		t.Fatal(snap.Error)
	}
}
