package service

import (
	"context"
	"fmt"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/domain"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"strings"
	"time"
)

type DeviceService struct {
	Repo interface {
		CreateDevice(context.Context, model.Device, []string) (model.Device, error)
		GetDevice(context.Context, int64) (model.Device, error)
		TransitionDevice(context.Context, int64, model.DeviceState, model.DeviceState, int64) error
		TouchDevice(context.Context, int64, time.Time) error
	}
	Clock model.Clock
}

func NewDevices(r interface {
	CreateDevice(context.Context, model.Device, []string) (model.Device, error)
	GetDevice(context.Context, int64) (model.Device, error)
	TransitionDevice(context.Context, int64, model.DeviceState, model.DeviceState, int64) error
	TouchDevice(context.Context, int64, time.Time) error
}, c model.Clock) *DeviceService {
	return &DeviceService{Repo: r, Clock: c}
}
func (s *DeviceService) Enroll(ctx context.Context, household int64, external string, kind model.DeviceKind, firmware string, caps []string) (model.Device, error) {
	external = strings.TrimSpace(external)
	firmware = strings.TrimSpace(firmware)
	if household <= 0 || domain.ValidateExternalID(external) != nil || firmware == "" || !validKind(kind) {
		return model.Device{}, model.ErrInvalid
	}
	normalized := make([]string, 0, len(caps))
	seen := make(map[string]struct{}, len(caps))
	for _, capability := range caps {
		capability = strings.ToLower(strings.TrimSpace(capability))
		if capability == "" {
			return model.Device{}, model.ErrInvalid
		}
		if _, exists := seen[capability]; exists {
			return model.Device{}, fmt.Errorf("%w: duplicate capability %s", model.ErrInvalid, capability)
		}
		seen[capability] = struct{}{}
		normalized = append(normalized, capability)
	}
	if !domain.HasCapabilities(domain.RequiredCapabilities(kind), normalized) {
		return model.Device{}, fmt.Errorf("%w: required device capabilities are missing", model.ErrInvalid)
	}
	return s.Repo.CreateDevice(ctx, model.Device{HouseholdID: household, ExternalID: external, Kind: kind, Firmware: firmware}, normalized)
}
func validKind(k model.DeviceKind) bool {
	switch k {
	case model.KindSensor, model.KindLight, model.KindThermostat, model.KindMeter, model.KindLock, model.KindController:
		return true
	default:
		return false
	}
}
func (s *DeviceService) Pair(ctx context.Context, id int64) error {
	d, err := s.Repo.GetDevice(ctx, id)
	if err != nil {
		return err
	}
	if d.State != model.DevicePending {
		return fmt.Errorf("%w: device is not pending", model.ErrConflict)
	}
	return s.Repo.TransitionDevice(ctx, id, model.DevicePending, model.DevicePaired, d.Version)
}
func (s *DeviceService) Enable(ctx context.Context, id int64) error {
	d, err := s.Repo.GetDevice(ctx, id)
	if err != nil {
		return err
	}
	if d.State != model.DevicePaired && d.State != model.DeviceDisabled {
		return fmt.Errorf("%w: device cannot be enabled", model.ErrConflict)
	}
	return s.Repo.TransitionDevice(ctx, id, d.State, model.DeviceEnabled, d.Version)
}
func (s *DeviceService) Disable(ctx context.Context, id int64) error {
	d, err := s.Repo.GetDevice(ctx, id)
	if err != nil {
		return err
	}
	if d.State != model.DeviceEnabled {
		return fmt.Errorf("%w: device cannot be disabled", model.ErrConflict)
	}
	return s.Repo.TransitionDevice(ctx, id, d.State, model.DeviceDisabled, d.Version)
}
func (s *DeviceService) Retire(ctx context.Context, id int64) error {
	d, err := s.Repo.GetDevice(ctx, id)
	if err != nil {
		return err
	}
	if d.State == model.DeviceRetired {
		return model.ErrConflict
	}
	return s.Repo.TransitionDevice(ctx, id, d.State, model.DeviceRetired, d.Version)
}
func (s *DeviceService) Touch(ctx context.Context, id int64) error {
	d, err := s.Repo.GetDevice(ctx, id)
	if err != nil {
		return err
	}
	now := s.Clock.Now()
	if now.Before(d.CreatedAt) {
		return model.ErrInvalid
	}
	return s.Repo.TouchDevice(ctx, id, now)
}
func DeviceFresh(d model.Device, now time.Time, ttl time.Duration) bool {
	return ttl > 0 && d.LastSeenAt != nil && !d.LastSeenAt.After(now) && now.Sub(*d.LastSeenAt) <= ttl
}
