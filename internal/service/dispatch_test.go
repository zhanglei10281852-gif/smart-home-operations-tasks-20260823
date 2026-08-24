package service

import (
	"context"
	"errors"
	"testing"

	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
)

type failingAuditStore struct {
	auditErr error
}

func (s *failingAuditStore) AddAudit(context.Context, model.AuditEvent) error    { return s.auditErr }
func (s *failingAuditStore) AddOutbox(context.Context, model.OutboxMessage) error { return nil }

// TestDispatchDoesNotReplayOnAuditFailure guards against replaying a command
// that the device has already accepted. When the audit write fails after a
// non-idempotent send succeeds, Dispatch must not call send a second time; it
// must surface the audit error directly so the external side effect is not
// duplicated.
func TestDispatchDoesNotReplayOnAuditFailure(t *testing.T) {
	f := &fakeRepo{device: model.Device{ID: 4, Kind: model.KindLight, State: model.DeviceEnabled}}
	devices := NewDevices(f, model.RealClock{})

	auditErr := errors.New("audit store unavailable")
	store := &failingAuditStore{auditErr: auditErr}

	dispatcher := NewDispatcher(devices, store, model.RealClock{})
	dispatcher.MaxBatch = 10

	sendCount := 0
	results, err := dispatcher.Dispatch(context.Background(), 1, nil, "req-1", []BatchCommand{{DeviceID: 4, Action: "on"}}, func(_ context.Context, command BatchCommand) error {
		sendCount++
		if command.Action != "on" {
			t.Fatalf("unexpected action %q", command.Action)
		}
		return nil
	})

	if sendCount != 1 {
		t.Fatalf("send invoked %d times, want exactly 1 (no replay on audit failure)", sendCount)
	}
	if len(results) != 1 || results[0].State != "accepted" {
		t.Fatalf("results=%+v", results)
	}
	if err == nil {
		t.Fatal("expected audit failure to propagate, got nil error")
	}
	if !errors.Is(err, auditErr) {
		t.Fatalf("expected audit error in chain, got %v", err)
	}
}
