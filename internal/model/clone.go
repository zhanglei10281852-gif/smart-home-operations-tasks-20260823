package model

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

func (d Device) Clone() Device {
	out := d
	if d.LastSeenAt != nil {
		value := *d.LastSeenAt
		out.LastSeenAt = &value
	}
	return out
}

func CloneDevices(input []Device) []Device {
	if input == nil {
		return nil
	}
	out := make([]Device, len(input))
	for i := range input {
		out[i] = input[i].Clone()
	}
	return out
}

func (t Telemetry) Validate(now time.Time) error {
	if t.DeviceID <= 0 || t.Sequence <= 0 || t.MeasuredAt.IsZero() || t.ReceivedAt.IsZero() {
		return errors.New("telemetry identity and timestamps are required")
	}
	if t.MeasuredAt.After(now.Add(5 * time.Minute)) {
		return errors.New("measurement is too far in the future")
	}
	if t.PowerWatts < 0 {
		return errors.New("power must be non-negative")
	}
	return nil
}

func (a AutomationAction) Validate() error {
	if a.AutomationID <= 0 || a.DeviceID <= 0 || strings.TrimSpace(a.Action) == "" || a.Ordinal < 0 {
		return errors.New("invalid automation action")
	}
	return nil
}

func (m OutboxMessage) Validate() error {
	if m.HouseholdID <= 0 || strings.TrimSpace(m.Topic) == "" || m.AvailableAt.IsZero() {
		return errors.New("invalid outbox message")
	}
	if len(m.Payload) > 1<<20 {
		return errors.New("outbox payload too large")
	}
	var value any
	if err := json.Unmarshal(m.Payload, &value); err != nil {
		return err
	}
	return nil
}

func NormalizeRole(role Role) (Role, error) {
	role = Role(strings.ToLower(strings.TrimSpace(string(role))))
	switch role {
	case RoleOwner, RoleOperator, RoleViewer:
		return role, nil
	default:
		return "", errors.New("unknown role")
	}
}

func (s Session) ActiveAt(now time.Time) bool {
	return !s.CreatedAt.After(now) && (s.RevokedAt == nil || s.RevokedAt.After(now)) && now.Before(s.ExpiresAt)
}

func (p EnergyPlan) Within(now time.Time) bool {
	return !p.StartsAt.After(now) && p.EndsAt.After(now) && p.State != PlanCancelled && p.State != PlanCompleted
}

func (a AuditEvent) Complete() bool {
	return a.HouseholdID > 0 && a.RequestID != "" && a.ObjectType != "" && a.ObjectID != "" && a.Action != ""
}

func (m Member) Can(role Role) bool {
	levels := map[Role]int{RoleViewer: 1, RoleOperator: 2, RoleOwner: 3}
	return m.Active && levels[m.Role] >= levels[role]
}

func (d Device) Operational() bool {
	return d.ID > 0 && d.State == DeviceEnabled && d.Version > 0
}

func (h Household) BudgetAvailable(spent int64) bool {
	return h.MonthlyBudgetCents >= 0 && spent >= 0 && spent <= h.MonthlyBudgetCents
}
