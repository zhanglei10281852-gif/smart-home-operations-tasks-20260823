package domain

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
)

type Transition struct {
	From, To string
	At       time.Time
	Actor    int64
	Reason   string
}

func ValidateTransition(current, next model.DeviceState) error {
	allowed := map[model.DeviceState]map[model.DeviceState]bool{model.DevicePending: {model.DevicePaired: true, model.DeviceRetired: true}, model.DevicePaired: {model.DeviceEnabled: true, model.DeviceRetired: true}, model.DeviceEnabled: {model.DeviceDisabled: true, model.DeviceRetired: true}, model.DeviceDisabled: {model.DeviceEnabled: true, model.DeviceRetired: true}, model.DeviceRetired: {}}
	if allowed[current][next] {
		return nil
	}
	return fmt.Errorf("%w: device %s cannot become %s", model.ErrConflict, current, next)
}
func ValidatePlanWindow(start, end, now time.Time) error {
	if start.IsZero() || end.IsZero() || !start.Before(end) {
		return model.ErrInvalid
	}
	if end.Sub(start) > 31*24*time.Hour {
		return errors.New("plan window exceeds one month")
	}
	if end.Before(now) {
		return errors.New("plan window is in the past")
	}
	return nil
}
func ValidateEmail(email string) error {
	email = strings.TrimSpace(email)
	at := strings.LastIndexByte(email, '@')
	if at < 1 || at == len(email)-1 || strings.ContainsAny(email, "\r\n\t ") {
		return model.ErrInvalid
	}
	return nil
}
func ValidateExternalID(id string) error {
	if id == "" || len(id) > 128 {
		return model.ErrInvalid
	}
	for _, r := range id {
		if r < ' ' || r == '\u007f' {
			return model.ErrInvalid
		}
	}
	return nil
}
func RequiredCapabilities(kind model.DeviceKind) []string {
	switch kind {
	case model.KindThermostat:
		return []string{"power", "temperature"}
	case model.KindLight:
		return []string{"power"}
	case model.KindLock:
		return []string{"lock"}
	case model.KindMeter:
		return []string{"power"}
	default:
		return nil
	}
}
func HasCapabilities(required, actual []string) bool {
	set := map[string]bool{}
	for _, v := range actual {
		set[v] = true
	}
	for _, v := range required {
		if !set[v] {
			return false
		}
	}
	return true
}
func CompatibleAction(kind model.DeviceKind, action string) bool {
	switch kind {
	case model.KindLight:
		return action == "on" || action == "off"
	case model.KindThermostat:
		return action == "heat" || action == "cool"
	case model.KindLock:
		return action == "lock" || action == "unlock"
	default:
		return false
	}
}
func ClampPower(watts, limit float64) float64 {
	if watts < 0 {
		return 0
	}
	if limit >= 0 && watts > limit {
		return limit
	}
	return watts
}
func SumPower(rows []model.PlanDevice) float64 {
	total := 0.0
	for _, row := range rows {
		total += row.TargetWatts
	}
	return total
}
func SortByTime(rows []model.Telemetry) []model.Telemetry {
	out := append([]model.Telemetry(nil), rows...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].MeasuredAt.Equal(out[j].MeasuredAt) {
			return out[i].Sequence < out[j].Sequence
		}
		return out[i].MeasuredAt.Before(out[j].MeasuredAt)
	})
	return out
}
func EnsureUnique[T comparable](values []T) error {
	seen := map[T]struct{}{}
	for _, value := range values {
		if _, ok := seen[value]; ok {
			return fmt.Errorf("%w: duplicate value", model.ErrInvalid)
		}
		seen[value] = struct{}{}
	}
	return nil
}
