package model

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound  = errors.New("record not found")
	ErrConflict  = errors.New("record conflict")
	ErrInvalid   = errors.New("invalid request")
	ErrForbidden = errors.New("operation forbidden")
	ErrBusy      = errors.New("resource busy")
)

type Role string

const (
	RoleOwner    Role = "owner"
	RoleOperator Role = "operator"
	RoleViewer   Role = "viewer"
)

type DeviceKind string

const (
	KindSensor     DeviceKind = "sensor"
	KindLight      DeviceKind = "light"
	KindThermostat DeviceKind = "thermostat"
	KindMeter      DeviceKind = "meter"
	KindLock       DeviceKind = "lock"
	KindController DeviceKind = "controller"
)

type DeviceState string

const (
	DevicePending  DeviceState = "pending"
	DevicePaired   DeviceState = "paired"
	DeviceEnabled  DeviceState = "enabled"
	DeviceDisabled DeviceState = "disabled"
	DeviceRetired  DeviceState = "retired"
)

type PlanState string

const (
	PlanDraft     PlanState = "draft"
	PlanScheduled PlanState = "scheduled"
	PlanRunning   PlanState = "running"
	PlanCompleted PlanState = "completed"
	PlanCancelled PlanState = "cancelled"
)

type AutomationState string

const (
	AutomationDraft    AutomationState = "draft"
	AutomationActive   AutomationState = "active"
	AutomationPaused   AutomationState = "paused"
	AutomationArchived AutomationState = "archived"
)

type RunState string

const (
	RunQueued    RunState = "queued"
	RunRunning   RunState = "running"
	RunSucceeded RunState = "succeeded"
	RunFailed    RunState = "failed"
	RunCancelled RunState = "cancelled"
)

type Household struct {
	ID                 int64
	Name, Timezone     string
	MonthlyBudgetCents int64
	CreatedAt          time.Time
}
type Member struct {
	ID, HouseholdID int64
	Email           string
	Role            Role
	Active          bool
	CreatedAt       time.Time
}
type Session struct {
	ID        string
	MemberID  int64
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}
type Device struct {
	ID, HouseholdID int64
	ExternalID      string
	Kind            DeviceKind
	State           DeviceState
	Firmware        string
	Version         int64
	LastSeenAt      *time.Time
	CreatedAt       time.Time
}
type Telemetry struct {
	ID, DeviceID, Sequence int64
	PowerWatts             float64
	TemperatureC           *float64
	MeasuredAt, ReceivedAt time.Time
}
type EnergyPlan struct {
	ID, HouseholdID  int64
	Name             string
	State            PlanState
	BudgetCents      int64
	StartsAt, EndsAt time.Time
	CreatedAt        time.Time
}
type PlanDevice struct {
	PlanID, DeviceID int64
	TargetWatts      float64
}
type Automation struct {
	ID, HouseholdID int64
	Name            string
	State           AutomationState
	TriggerKind     string
	CreatedAt       time.Time
}
type AutomationAction struct {
	ID, AutomationID, DeviceID int64
	Action                     string
	Ordinal                    int
}
type AutomationRun struct {
	ID, AutomationID      int64
	IdempotencyKey        string
	State                 RunState
	ErrorText             string
	StartedAt, FinishedAt *time.Time
}
type Alert struct {
	ID, HouseholdID       int64
	DeviceID              *int64
	Severity, Code, State string
	Details               []byte
	CreatedAt             time.Time
	ResolvedAt            *time.Time
}
type AuditEvent struct {
	ID, HouseholdID                         int64
	ActorMemberID                           *int64
	RequestID, ObjectType, ObjectID, Action string
	Payload                                 []byte
	CreatedAt                               time.Time
}
type OutboxMessage struct {
	ID, HouseholdID       int64
	Topic                 string
	Payload               []byte
	Attempts              int
	AvailableAt           time.Time
	LockedAt, DeliveredAt *time.Time
}

type Clock interface{ Now() time.Time }
type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now().UTC() }

type TxFunc func(context.Context) error
type TransactionRunner interface {
	WithTx(context.Context, TxFunc) error
}
