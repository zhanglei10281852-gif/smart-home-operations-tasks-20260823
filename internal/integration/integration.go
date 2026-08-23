package integration

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

type DeviceGateway interface {
	Command(context.Context, string, string, map[string]any) error
	Health(context.Context, string) error
}
type Result struct {
	ID       string
	Accepted bool
	Error    string
	At       time.Time
}
type Dispatcher struct {
	Gateway DeviceGateway
	Mu      sync.Mutex
	Results map[string]Result
	Timeout time.Duration
}

func NewDispatcher(g DeviceGateway) *Dispatcher {
	return &Dispatcher{Gateway: g, Results: map[string]Result{}, Timeout: 5 * time.Second}
}
func (d *Dispatcher) Dispatch(ctx context.Context, id, action string, payload map[string]any) Result {
	d.Mu.Lock()
	if previous, ok := d.Results[id]; ok {
		d.Mu.Unlock()
		return previous
	}
	d.Mu.Unlock()
	if d.Timeout <= 0 {
		d.Timeout = 5 * time.Second
	}
	callCtx, cancel := context.WithTimeout(ctx, d.Timeout)
	defer cancel()
	err := d.Gateway.Command(callCtx, id, action, payload)
	result := Result{ID: id, Accepted: err == nil, At: time.Now().UTC()}
	if err != nil {
		result.Error = err.Error()
	}
	d.Mu.Lock()
	d.Results[id] = result
	d.Mu.Unlock()
	return result
}
func (d *Dispatcher) Forget(id string) { d.Mu.Lock(); defer d.Mu.Unlock(); delete(d.Results, id) }
func EncodeCommand(id, action string, payload map[string]any) ([]byte, error) {
	return json.Marshal(map[string]any{"id": id, "action": action, "payload": payload})
}

type HealthSnapshot struct {
	DeviceID  string
	Healthy   bool
	Error     string
	CheckedAt time.Time
}

func CheckGateway(ctx context.Context, g DeviceGateway, id string) HealthSnapshot {
	err := g.Health(ctx, id)
	snapshot := HealthSnapshot{DeviceID: id, Healthy: err == nil, CheckedAt: time.Now().UTC()}
	if err != nil {
		snapshot.Error = err.Error()
	}
	return snapshot
}
func IsTransient(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
