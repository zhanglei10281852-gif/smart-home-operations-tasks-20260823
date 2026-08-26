package integration

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type gateway struct {
	mu      sync.Mutex
	calls   int
	err     error
	start   chan struct{}
	release chan struct{}
}

func (g *gateway) Command(context.Context, string, string, map[string]any) error {
	g.mu.Lock()
	g.calls++
	g.mu.Unlock()
	if g.start != nil {
		select {
		case g.start <- struct{}{}:
		default:
		}
	}
	if g.release != nil {
		<-g.release
	}
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

func TestDispatcherCoalescesConcurrentIdempotentCommands(t *testing.T) {
	g := &gateway{start: make(chan struct{}, 1), release: make(chan struct{})}
	d := NewDispatcher(g)
	results := make(chan Result, 2)
	go func() { results <- d.Dispatch(context.Background(), "same-key", "on", map[string]any{"level": 1}) }()
	<-g.start
	go func() { results <- d.Dispatch(context.Background(), "same-key", "on", map[string]any{"level": 1}) }()
	close(g.release)
	first, second := <-results, <-results
	g.mu.Lock()
	calls := g.calls
	g.mu.Unlock()
	if calls != 1 || !first.Accepted || !second.Accepted {
		t.Fatalf("calls=%d first=%+v second=%+v", calls, first, second)
	}
	conflict := d.Dispatch(context.Background(), "same-key", "off", map[string]any{"level": 1})
	if conflict.Accepted || conflict.Error == "" {
		t.Fatalf("idempotency conflict=%+v", conflict)
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
