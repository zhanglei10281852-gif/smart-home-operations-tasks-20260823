package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
)

type dispatchDeviceRepo060 struct {
	device model.Device
}

func (r *dispatchDeviceRepo060) CreateDevice(context.Context, model.Device, []string) (model.Device, error) {
	return r.device, nil
}
func (r *dispatchDeviceRepo060) GetDevice(context.Context, int64) (model.Device, error) {
	return r.device, nil
}
func (r *dispatchDeviceRepo060) TransitionDevice(context.Context, int64, model.DeviceState, model.DeviceState, int64) error {
	return nil
}
func (r *dispatchDeviceRepo060) TouchDevice(context.Context, int64, time.Time) error { return nil }

type failingAuditStore060 struct {
	err error
}

func (s *failingAuditStore060) AddAudit(context.Context, model.AuditEvent) error {
	return s.err
}
func (s *failingAuditStore060) AddOutbox(context.Context, model.OutboxMessage) error {
	return nil
}

func TestAuditFailureDoesNotReplayAcceptedDeviceCommand(t *testing.T) {
	devices := NewDevices(&dispatchDeviceRepo060{device: model.Device{
		ID:    60,
		Kind:  model.KindLight,
		State: model.DeviceEnabled,
	}}, FixedClock{Value: time.Now().UTC()})
	auditFailure := errors.New("audit database unavailable")
	dispatcher := NewDispatcher(devices, &failingAuditStore060{err: auditFailure}, FixedClock{Value: time.Now().UTC()})
	var sends atomic.Int32
	records, err := dispatcher.Dispatch(context.Background(), 1, nil, "request-060", []BatchCommand{{
		DeviceID: 60,
		Action:   "on",
	}}, func(context.Context, BatchCommand) error {
		sends.Add(1)
		return nil
	})
	if !errors.Is(err, auditFailure) {
		t.Fatalf("dispatch err=%v want audit failure", err)
	}
	if len(records) != 1 || records[0].State != "accepted" {
		t.Fatalf("records=%+v", records)
	}
	if sends.Load() != 1 {
		t.Fatalf("non-idempotent device command sent %d times after audit failure", sends.Load())
	}
}
