package model

import (
	"testing"
	"time"
)

func TestCloneDevicesAndValidation(t *testing.T) {
	seen := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	devices := []Device{{ID: 1, ExternalID: "lamp", LastSeenAt: &seen}}
	copyDevices := CloneDevices(devices)
	if len(copyDevices) != 1 || copyDevices[0].ID != 1 || copyDevices[0].LastSeenAt == devices[0].LastSeenAt {
		t.Fatalf("copy=%+v", copyDevices)
	}
	copyDevices[0].ExternalID = "changed"
	if devices[0].ExternalID == "changed" {
		t.Fatal("device slice leaked")
	}
	if _, err := NormalizeRole(" OPERATOR "); err != nil {
		t.Fatal(err)
	}
	if _, err := NormalizeRole("admin"); err == nil {
		t.Fatal("unknown role accepted")
	}
}

func TestTelemetryAndOutboxValidation(t *testing.T) {
	now := time.Now().UTC()
	if err := (Telemetry{DeviceID: 1, Sequence: 1, MeasuredAt: now, ReceivedAt: now, PowerWatts: 4}).Validate(now); err != nil {
		t.Fatal(err)
	}
	if err := (Telemetry{DeviceID: 1, Sequence: 1, MeasuredAt: now, ReceivedAt: now, PowerWatts: -1}).Validate(now); err == nil {
		t.Fatal("negative telemetry accepted")
	}
	if err := (OutboxMessage{HouseholdID: 1, Topic: "device.updated", Payload: []byte(`{"ok":true}`), AvailableAt: now}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (OutboxMessage{HouseholdID: 1, Topic: "device.updated", Payload: []byte(`bad`), AvailableAt: now}).Validate(); err == nil {
		t.Fatal("invalid payload accepted")
	}
}
