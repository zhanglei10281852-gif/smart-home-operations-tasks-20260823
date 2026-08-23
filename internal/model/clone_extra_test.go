package model

import (
	"testing"
	"time"
)

func TestSessionAndPlanLifecyclePredicates(t *testing.T) {
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	session := Session{ID: "session-1", MemberID: 1, CreatedAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour)}
	if !session.ActiveAt(now) {
		t.Fatal("active session rejected")
	}
	revoked := now.Add(-time.Minute)
	session.RevokedAt = &revoked
	if session.ActiveAt(now) {
		t.Fatal("revoked session accepted")
	}
	plan := EnergyPlan{StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour), State: PlanRunning}
	if !plan.Within(now) {
		t.Fatal("running plan outside window")
	}
	plan.State = PlanCompleted
	if plan.Within(now) {
		t.Fatal("completed plan considered active")
	}
}

func TestAuditCompleteness(t *testing.T) {
	if (AuditEvent{HouseholdID: 1, RequestID: "r", ObjectType: "device", ObjectID: "1", Action: "enabled"}).Complete() == false {
		t.Fatal("complete audit rejected")
	}
	for _, event := range []AuditEvent{{}, {HouseholdID: 1}, {HouseholdID: 1, RequestID: "r", ObjectType: "device"}} {
		if event.Complete() {
			t.Fatalf("incomplete audit accepted: %+v", event)
		}
	}
}

func TestMemberCanRole(t *testing.T) {
	member := Member{Role: RoleOperator, Active: true}
	if !member.Can(RoleViewer) || !member.Can(RoleOperator) || member.Can(RoleOwner) {
		t.Fatal("role hierarchy incorrect")
	}
	member.Active = false
	if member.Can(RoleViewer) {
		t.Fatal("inactive member authorized")
	}
	member.Active = true
	if member.Can(Role("administrator")) || (Member{Role: Role("administrator"), Active: true}).Can(RoleViewer) {
		t.Fatal("unknown role entered the hierarchy")
	}
}

func TestDeviceOperational(t *testing.T) {
	if !(Device{ID: 1, State: DeviceEnabled, Version: 1}).Operational() {
		t.Fatal("enabled device not operational")
	}
	for _, device := range []Device{{ID: 1, State: DeviceDisabled, Version: 1}, {ID: 1, State: DeviceEnabled}, {State: DeviceEnabled, Version: 1}} {
		if device.Operational() {
			t.Fatalf("non-operational device accepted: %+v", device)
		}
	}
}

func TestHouseholdBudgetAvailable(t *testing.T) {
	h := Household{MonthlyBudgetCents: 100}
	if !h.BudgetAvailable(100) || h.BudgetAvailable(101) || h.BudgetAvailable(-1) {
		t.Fatal("budget boundary incorrect")
	}
}
