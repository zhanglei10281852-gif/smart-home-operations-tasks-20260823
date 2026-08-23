package service

import (
	"context"
	"errors"
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
	if household <= 0 || external == "" || firmware == "" || !validKind(kind) {
		return model.Device{}, model.ErrInvalid
	}
	return s.Repo.CreateDevice(ctx, model.Device{HouseholdID: household, ExternalID: external, Kind: kind, Firmware: firmware}, caps)
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
		return errors.New("device is not pending")
	}
	return s.Repo.TransitionDevice(ctx, id, model.DevicePending, model.DevicePaired, d.Version)
}
func (s *DeviceService) Enable(ctx context.Context, id int64) error {
	d, err := s.Repo.GetDevice(ctx, id)
	if err != nil {
		return err
	}
	if d.State != model.DevicePaired && d.State != model.DeviceDisabled {
		return errors.New("device cannot be enabled")
	}
	return s.Repo.TransitionDevice(ctx, id, d.State, model.DeviceEnabled, d.Version)
}
func (s *DeviceService) Disable(ctx context.Context, id int64) error {
	d, err := s.Repo.GetDevice(ctx, id)
	if err != nil {
		return err
	}
	if d.State != model.DeviceEnabled {
		return errors.New("device cannot be disabled")
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
	return d.LastSeenAt != nil && now.Sub(*d.LastSeenAt) <= ttl
}
