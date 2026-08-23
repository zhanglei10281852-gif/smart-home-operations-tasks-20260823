package domain

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
)

func TestValidateDeviceTransitions(t *testing.T) {
	cases := []struct {
		name     string
		from, to model.DeviceState
		want     bool
	}{
		{"pending-paired", model.DevicePending, model.DevicePaired, true},
		{"pending-retired", model.DevicePending, model.DeviceRetired, true},
		{"paired-enabled", model.DevicePaired, model.DeviceEnabled, true},
		{"paired-retired", model.DevicePaired, model.DeviceRetired, true},
		{"enabled-disabled", model.DeviceEnabled, model.DeviceDisabled, true},
		{"enabled-retired", model.DeviceEnabled, model.DeviceRetired, true},
		{"disabled-enabled", model.DeviceDisabled, model.DeviceEnabled, true},
		{"disabled-retired", model.DeviceDisabled, model.DeviceRetired, true},
		{"retired-any", model.DeviceRetired, model.DeviceEnabled, false},
		{"pending-disabled", model.DevicePending, model.DeviceDisabled, false},
		{"enabled-pending", model.DeviceEnabled, model.DevicePending, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateTransition(tc.from, tc.to)
			if (err == nil) != tc.want {
				t.Fatalf("err=%v want=%v", err, tc.want)
			}
		})
	}
}

func TestValidatePlanWindow(t *testing.T) {
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		start, end time.Time
		wantErr    bool
	}{
		{"valid", now.Add(time.Hour), now.Add(2 * time.Hour), false},
		{"zero-start", time.Time{}, now, true},
		{"zero-end", now, time.Time{}, true},
		{"reversed", now.Add(time.Hour), now, true},
		{"too-long", now.Add(time.Hour), now.Add(32 * 24 * time.Hour), true},
		{"past", now.Add(-2 * time.Hour), now.Add(-time.Hour), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidatePlanWindow(tc.start, tc.end, now); (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestValidationTextRules(t *testing.T) {
	for _, email := range []string{"a@example.com", "operator+1@example.org", "first.last@example.co"} {
		if err := ValidateEmail(email); err != nil {
			t.Errorf("valid email %q: %v", email, err)
		}
	}
	for _, email := range []string{"", "missing-at", "@missing-user.com", "user@", "user\n@example.com", "user @example.com"} {
		if err := ValidateEmail(email); err == nil {
			t.Errorf("invalid email accepted %q", email)
		}
	}
	for _, id := range []string{"plug-1", "meter/garage", "A_123"} {
		if err := ValidateExternalID(id); err != nil {
			t.Errorf("valid id %q: %v", id, err)
		}
	}
	for _, id := range []string{"", "bad\nidentifier", "bad\tidentifier", string(make([]byte, 129))} {
		if err := ValidateExternalID(id); err == nil {
			t.Errorf("invalid id accepted %q", id)
		}
	}
}

func TestCapabilitiesAndActions(t *testing.T) {
	tests := []struct {
		kind     model.DeviceKind
		required []string
	}{
		{model.KindThermostat, []string{"power", "temperature"}},
		{model.KindLight, []string{"power"}},
		{model.KindLock, []string{"lock"}},
		{model.KindMeter, []string{"power"}},
	}
	for _, tc := range tests {
		if got := RequiredCapabilities(tc.kind); fmt.Sprint(got) != fmt.Sprint(tc.required) {
			t.Errorf("%s capabilities=%v want=%v", tc.kind, got, tc.required)
		}
	}
	if !HasCapabilities([]string{"power", "temperature"}, []string{"temperature", "power", "battery"}) {
		t.Fatal("complete capabilities rejected")
	}
	if HasCapabilities([]string{"power", "temperature"}, []string{"power"}) {
		t.Fatal("incomplete capabilities accepted")
	}
	for _, tc := range []struct {
		kind   model.DeviceKind
		action string
		want   bool
	}{{model.KindLight, "on", true}, {model.KindLight, "heat", false}, {model.KindThermostat, "heat", true}, {model.KindThermostat, "on", false}, {model.KindLock, "unlock", true}, {model.KindMeter, "read", false}} {
		if got := CompatibleAction(tc.kind, tc.action); got != tc.want {
			t.Errorf("%s/%s=%v want=%v", tc.kind, tc.action, got, tc.want)
		}
	}
}

func TestPowerAndScheduleRules(t *testing.T) {
	if ClampPower(-1, 10) != 0 {
		t.Fatal("negative power was not clamped")
	}
	if ClampPower(50, 10) != 10 {
		t.Fatal("power cap ignored")
	}
	if ClampPower(5, 10) != 5 {
		t.Fatal("valid power changed")
	}
	base := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)
	rows := []ScheduleWindow{{DeviceID: 1, Start: base, End: base.Add(time.Hour), Watts: 10}, {DeviceID: 1, Start: base.Add(30 * time.Minute), End: base.Add(90 * time.Minute), Watts: 5}, {DeviceID: 2, Start: base.Add(30 * time.Minute), End: base.Add(time.Hour), Watts: 99}}
	if len(Conflicts(rows)) != 1 {
		t.Fatalf("conflicts=%v", Conflicts(rows))
	}
	if err := Capacity(rows, 15); err == nil {
		t.Fatal("capacity overflow accepted")
	}
	if err := Capacity(rows[:2], 20); err != nil {
		t.Fatalf("valid capacity rejected: %v", err)
	}
	if err := (ScheduleWindow{DeviceID: 0, Start: base, End: base, Watts: -1}).Valid(); err == nil {
		t.Fatal("invalid window accepted")
	}
}

func TestPaginationAndUniqueness(t *testing.T) {
	p := NormalizePage(Page{Limit: 0, Offset: -10, Query: "  lamps "})
	if p.Limit != 50 || p.Offset != 0 || p.Query != "lamps" {
		t.Fatalf("normalized page=%+v", p)
	}
	if err := ValidatePage(Page{Limit: 201}); err == nil {
		t.Fatal("oversized page accepted")
	}
	if err := ValidatePage(Page{Limit: 20, Offset: 0}); err != nil {
		t.Fatalf("valid page rejected: %v", err)
	}
	if err := EnsureUnique([]string{"a", "b", "c"}); err != nil {
		t.Fatal(err)
	}
	if err := EnsureUnique([]string{"a", "b", "a"}); err == nil {
		t.Fatal("duplicate values accepted")
	}
	result := NewResult([]int{1, 2}, 4, Page{Limit: 2, Offset: 2})
	if result.Total != 4 || len(result.Items) != 2 || result.Offset != 2 {
		t.Fatalf("result=%+v", result)
	}
}

func TestEventFanoutAndNotifications(t *testing.T) {
	good := Event{Topic: "device.updated", HouseholdID: 1, ObjectType: "device", ObjectID: "7", Action: "enabled", RequestID: "req-1", At: time.Now()}
	if _, err := good.JSON(); err != nil {
		t.Fatal(err)
	}
	if err := (Event{}).Validate(); err == nil {
		t.Fatal("empty event accepted")
	}
	consumer := &testConsumer{}
	fanout := Fanout{Consumers: []EventConsumer{consumer, consumer}}
	if err := fanout.Publish(good); err != nil {
		t.Fatal(err)
	}
	if consumer.count != 2 {
		t.Fatalf("consumer count=%d", consumer.count)
	}
	if err := ValidateNotification(Notification{Recipient: "x", Channel: "push", Subject: "subject", Body: "body"}); err != nil {
		t.Fatal(err)
	}
}

type testConsumer struct {
	mu    sync.Mutex
	count int
}

func (c *testConsumer) Consume(Event) error { c.mu.Lock(); defer c.mu.Unlock(); c.count++; return nil }
func TestParallelHelpers(t *testing.T) {
	ctx := context.Background()
	values := []int{1, 2, 3, 4, 5}
	errs := Parallel(ctx, values, func(_ context.Context, v int) error {
		if v == 4 {
			return errors.New("four")
		}
		return nil
	})
	if FirstError(errs) == nil {
		t.Fatal("first error missing")
	}
	if RequireAll(errs) == nil {
		t.Fatal("batch error missing")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	errs = Parallel(ctx, values, func(_ context.Context, _ int) error { return nil })
	for _, err := range errs {
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancel error=%v", err)
		}
	}
}
