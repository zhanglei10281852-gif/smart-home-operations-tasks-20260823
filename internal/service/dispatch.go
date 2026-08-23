package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
)

type DispatchRecord struct {
	RequestID string
	DeviceID  int64
	Action    string
	State     string
	Error     string
	At        time.Time
}

type DispatchStore interface {
	AddAudit(context.Context, model.AuditEvent) error
	AddOutbox(context.Context, model.OutboxMessage) error
}

type Dispatcher struct {
	Devices  *DeviceService
	Store    DispatchStore
	Clock    model.Clock
	MaxBatch int
}

func NewDispatcher(devices *DeviceService, store DispatchStore, clock model.Clock) *Dispatcher {
	return &Dispatcher{Devices: devices, Store: store, Clock: clock, MaxBatch: 50}
}

func (d *Dispatcher) Dispatch(ctx context.Context, household int64, member *int64, requestID string, commands []BatchCommand, send func(context.Context, BatchCommand) error) ([]DispatchRecord, error) {
	if household <= 0 || strings.TrimSpace(requestID) == "" || len(commands) == 0 || len(commands) > d.MaxBatch {
		return nil, model.ErrInvalid
	}
	if err := d.validate(ctx, commands); err != nil {
		return nil, err
	}
	results := make([]DispatchRecord, len(commands))
	for i, command := range commands {
		record := DispatchRecord{RequestID: requestID, DeviceID: command.DeviceID, Action: command.Action, At: d.Clock.Now()}
		if err := ctx.Err(); err != nil {
			record.State, record.Error = "cancelled", err.Error()
			results[i] = record
			return results, err
		}
		err := send(ctx, command)
		if err != nil {
			record.State, record.Error = "failed", err.Error()
		} else {
			record.State = "accepted"
		}
		results[i] = record
		if d.Store != nil {
			auditErr := d.Store.AddAudit(ctx, model.AuditEvent{HouseholdID: household, ActorMemberID: member, RequestID: requestID, ObjectType: "device", ObjectID: fmt.Sprint(command.DeviceID), Action: record.State})
			if auditErr != nil {
				return results, fmt.Errorf("audit dispatch: %w", auditErr)
			}
		}
	}
	return results, nil
}

func (d *Dispatcher) validate(ctx context.Context, commands []BatchCommand) error {
	if d.Devices == nil || d.Devices.Repo == nil {
		return errors.New("device service is not configured")
	}
	for _, command := range commands {
		device, err := d.Devices.Repo.GetDevice(ctx, command.DeviceID)
		if err != nil {
			return err
		}
		if err := ValidateDeviceCommand(device, command.Action); err != nil {
			return err
		}
	}
	return nil
}

func (d *Dispatcher) Summarize(records []DispatchRecord) (accepted, failed int) {
	for _, record := range records {
		switch record.State {
		case "accepted":
			accepted++
		case "failed", "cancelled":
			failed++
		}
	}
	return accepted, failed
}

func ValidateDispatchRecords(records []DispatchRecord) error {
	seen := make(map[int64]struct{}, len(records))
	for _, record := range records {
		if record.DeviceID <= 0 || record.RequestID == "" || record.At.IsZero() {
			return errors.New("dispatch record is incomplete")
		}
		if _, ok := seen[record.DeviceID]; ok {
			return errors.New("duplicate dispatch record")
		}
		seen[record.DeviceID] = struct{}{}
		if record.State != "accepted" && record.State != "failed" && record.State != "cancelled" {
			return errors.New("unknown dispatch state")
		}
	}
	return nil
}
