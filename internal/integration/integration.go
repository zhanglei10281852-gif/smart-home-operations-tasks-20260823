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
	Gateway      DeviceGateway
	Mu           sync.Mutex
	Results      map[string]Result
	Timeout      time.Duration
	fingerprints map[string]string
	inflight     map[string]*dispatchCall
}

type dispatchCall struct {
	done        chan struct{}
	result      Result
	fingerprint string
}

func NewDispatcher(g DeviceGateway) *Dispatcher {
	return &Dispatcher{Gateway: g, Results: map[string]Result{}, Timeout: 5 * time.Second, fingerprints: map[string]string{}, inflight: map[string]*dispatchCall{}}
}
func (d *Dispatcher) Dispatch(ctx context.Context, id, action string, payload map[string]any) Result {
	if d.Gateway == nil || id == "" || action == "" {
		return Result{ID: id, Error: "dispatcher request is incomplete", At: time.Now().UTC()}
	}
	encoded, err := EncodeCommand(id, action, payload)
	if err != nil {
		return Result{ID: id, Error: err.Error(), At: time.Now().UTC()}
	}
	fingerprint := string(encoded)
	d.Mu.Lock()
	if previous, ok := d.Results[id]; ok {
		if d.fingerprints[id] != fingerprint {
			d.Mu.Unlock()
			return Result{ID: id, Error: "idempotency key was reused with a different command", At: time.Now().UTC()}
		}
		d.Mu.Unlock()
		return previous
	}
	if call := d.inflight[id]; call != nil {
		if call.fingerprint != fingerprint {
			d.Mu.Unlock()
			return Result{ID: id, Error: "idempotency key was reused with a different command", At: time.Now().UTC()}
		}
		d.Mu.Unlock()
		select {
		case <-ctx.Done():
			return Result{ID: id, Error: ctx.Err().Error(), At: time.Now().UTC()}
		case <-call.done:
			return call.result
		}
	}
	call := &dispatchCall{done: make(chan struct{}), fingerprint: fingerprint}
	if d.inflight == nil {
		d.inflight = make(map[string]*dispatchCall)
	}
	if d.Results == nil {
		d.Results = make(map[string]Result)
	}
	if d.fingerprints == nil {
		d.fingerprints = make(map[string]string)
	}
	d.inflight[id] = call
	timeout := d.Timeout
	d.Mu.Unlock()
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	err = d.Gateway.Command(callCtx, id, action, payload)
	result := Result{ID: id, Accepted: err == nil, At: time.Now().UTC()}
	if err != nil {
		result.Error = err.Error()
	}
	d.Mu.Lock()
	d.Results[id] = result
	d.fingerprints[id] = fingerprint
	call.result = result
	delete(d.inflight, id)
	close(call.done)
	d.Mu.Unlock()
	return result
}
func (d *Dispatcher) Forget(id string) {
	d.Mu.Lock()
	defer d.Mu.Unlock()
	delete(d.Results, id)
	delete(d.fingerprints, id)
}
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
	if g == nil || id == "" {
		return HealthSnapshot{DeviceID: id, Error: "gateway check is not configured", CheckedAt: time.Now().UTC()}
	}
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
