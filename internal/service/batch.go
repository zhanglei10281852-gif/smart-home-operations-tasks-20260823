package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/zhanglei10281852-gif/smart-home-operations/internal/domain"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
)

type BatchRepository interface {
	GetDevice(context.Context, int64) (model.Device, error)
	AddAudit(context.Context, model.AuditEvent) error
	AddOutbox(context.Context, model.OutboxMessage) error
	TransitionWithAudit(context.Context, interface{}) error
}

type BatchCommand struct {
	DeviceID  int64
	Action    string
	RequestID string
	Payload   map[string]any
}

type BatchService struct {
	Devices *DeviceService
	Audit   *AuditService
	Clock   model.Clock
}

func NewBatch(devices *DeviceService, audit *AuditService, clock model.Clock) *BatchService {
	return &BatchService{Devices: devices, Audit: audit, Clock: clock}
}

// ValidateCommands applies device-specific action rules before dispatch.
func (s *BatchService) ValidateCommands(ctx context.Context, commands []BatchCommand) error {
	items := make([]domain.BatchItem, len(commands))
	for i, command := range commands {
		items[i] = domain.BatchItem{DeviceID: command.DeviceID, Action: command.Action, Payload: command.Payload}
	}
	if err := domain.ValidateBatch(items); err != nil {
		return err
	}
	for _, command := range commands {
		device, err := s.Devices.Repo.GetDevice(ctx, command.DeviceID)
		if err != nil {
			return fmt.Errorf("load device %d: %w", command.DeviceID, err)
		}
		if err := ValidateDeviceCommand(device, command.Action); err != nil {
			return fmt.Errorf("device %d: %w", command.DeviceID, err)
		}
	}
	return nil
}

func ValidateDeviceCommand(device model.Device, action string) error {
	if device.State != model.DeviceEnabled {
		return errors.New("device is not enabled")
	}
	switch device.Kind {
	case model.KindLight:
		if action != "on" && action != "off" {
			return model.ErrInvalid
		}
	case model.KindThermostat:
		if action != "heat" && action != "cool" {
			return model.ErrInvalid
		}
	case model.KindLock:
		if action != "lock" && action != "unlock" {
			return model.ErrInvalid
		}
	default:
		return errors.New("device does not accept commands")
	}
	return nil
}

func (s *BatchService) Execute(ctx context.Context, commands []BatchCommand, dispatch func(context.Context, BatchCommand) error) ([]domain.BatchResult, error) {
	if err := s.ValidateCommands(ctx, commands); err != nil {
		return nil, err
	}
	items := make([]domain.BatchItem, len(commands))
	for i, command := range commands {
		items[i] = domain.BatchItem{DeviceID: command.DeviceID, Action: command.Action, Payload: command.Payload}
	}
	return domain.ExecuteBatch(ctx, items, func(ctx context.Context, item domain.BatchItem) error {
		for _, command := range commands {
			if command.DeviceID == item.DeviceID {
				return dispatch(ctx, command)
			}
		}
		return errors.New("batch command disappeared")
	})
}

func (s *BatchService) AuditResult(ctx context.Context, household int64, member *int64, requestID string, result domain.BatchResult) error {
	if result.DeviceID <= 0 || requestID == "" {
		return model.ErrInvalid
	}
	action := "accepted"
	if result.Error != nil {
		action = "failed"
	}
	return s.Audit.Record(ctx, household, member, requestID, "device", fmt.Sprint(result.DeviceID), action, map[string]any{"error": errorText(result.Error), "finished_at": result.Finished.UTC().Format(time.RFC3339Nano)})
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
