package db_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/zhanglei10281852-gif/smart-home-operations/internal/db"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/repo"
)

func TestPostgresMigrationTransactionAndRestart(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://smart_home:smart_home@127.0.0.1:55432/smart_home?sslmode=disable"
	}
	database, err := db.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.Migrate(ctx, database); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx, database); err != nil {
		t.Fatal("migration is not repeatable: ", err)
	}
	migrationResults := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { migrationResults <- db.Migrate(ctx, database) }()
	}
	for i := 0; i < 2; i++ {
		if err := <-migrationResults; err != nil {
			t.Fatal("concurrent migration failed: ", err)
		}
	}
	versions, err := db.MigrationVersions(ctx, database)
	if err != nil || len(versions) == 0 {
		t.Fatalf("versions=%v err=%v", versions, err)
	}
	_, err = database.SQL.ExecContext(ctx, `TRUNCATE audit_events,outbox_messages,automation_runs,automation_actions,automations,plan_devices,energy_plans,telemetry,device_capabilities,devices,sessions,members,households RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatal(err)
	}
	r := repo.New(database)
	if _, err := database.SQL.ExecContext(ctx, `CREATE OR REPLACE FUNCTION reject_blocked_owner() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.email='blocked@example.com' THEN RAISE EXCEPTION 'blocked owner'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_blocked_owner BEFORE INSERT ON members FOR EACH ROW EXECUTE FUNCTION reject_blocked_owner()`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.CreateHouseholdWithOwner(ctx, "rollback-on-owner", "UTC", 100, "blocked@example.com", "hash"); err == nil {
		t.Fatal("owner failure unexpectedly committed household")
	}
	var count int
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM households WHERE name='rollback-on-owner'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("onboarding rollback count=%d err=%v", count, err)
	}
	if _, err := database.SQL.ExecContext(ctx, `DROP TRIGGER reject_blocked_owner ON members; DROP FUNCTION reject_blocked_owner()`); err != nil {
		t.Fatal(err)
	}
	household, err := r.CreateHousehold(ctx, "family", "Asia/Shanghai", 5000)
	if err != nil {
		t.Fatal(err)
	}
	member, err := r.AddMember(ctx, household.ID, "owner@example.com", "hash", model.RoleOwner)
	if err != nil {
		t.Fatal(err)
	}
	device, err := r.CreateDevice(ctx, model.Device{HouseholdID: household.ID, ExternalID: "lamp", Kind: model.KindLight, Firmware: "1.0"}, []string{"power"})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.TransitionDevice(ctx, device.ID, model.DevicePending, model.DevicePaired, device.Version); err != nil {
		t.Fatal(err)
	}
	if err := r.TransitionDevice(ctx, device.ID, model.DevicePaired, model.DeviceEnabled, device.Version+1); err != nil {
		t.Fatal(err)
	}
	if err := r.InsertTelemetry(ctx, model.Telemetry{DeviceID: device.ID, Sequence: 1, PowerWatts: 12, MeasuredAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	plan, err := r.CreatePlan(ctx, model.EnergyPlan{HouseholdID: household.ID, Name: "night", BudgetCents: 100, StartsAt: time.Now().UTC().Add(time.Hour), EndsAt: time.Now().UTC().Add(2 * time.Hour)}, []model.PlanDevice{{DeviceID: device.ID, TargetWatts: 20}})
	if err != nil || plan.ID == 0 {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
	automation, err := r.CreateAutomation(ctx, model.Automation{HouseholdID: household.ID, Name: "arrive", TriggerKind: "presence"}, []model.AutomationAction{{DeviceID: device.ID, Action: "on", Ordinal: 0}})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.SetAutomationState(ctx, automation.ID, model.AutomationDraft, model.AutomationActive); err != nil {
		t.Fatal(err)
	}
	run, err := r.QueueRun(ctx, automation.ID, "idempotent-key")
	if err != nil {
		t.Fatal(err)
	}
	same, err := r.QueueRun(ctx, automation.ID, "idempotent-key")
	if err != nil || same.ID != run.ID {
		t.Fatalf("idempotency run=%+v same=%+v err=%v", run, same, err)
	}
	claimedRun, err := r.ClaimRun(ctx)
	if err != nil || claimedRun.ID != run.ID {
		t.Fatalf("claimed run=%+v err=%v", claimedRun, err)
	}
	if err := r.ExecuteAutomationRun(ctx, claimedRun.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_messages WHERE topic='device.command' AND payload->>'run_id'=$1`, fmt.Sprint(run.ID)).Scan(&count); err != nil || count != 1 {
		t.Fatalf("automation outbox count=%d err=%v", count, err)
	}
	if _, err := database.SQL.ExecContext(ctx, `UPDATE outbox_messages SET delivered_at=now() WHERE topic='device.command'`); err != nil {
		t.Fatal(err)
	}
	if err := r.SetPlanState(ctx, plan.ID, model.PlanDraft, model.PlanScheduled); err != nil {
		t.Fatal(err)
	}
	if err := r.TransitionDevice(ctx, device.ID, model.DeviceEnabled, model.DeviceDisabled, device.Version+2); err != nil {
		t.Fatal(err)
	}
	if err := r.InsertTelemetry(ctx, model.Telemetry{DeviceID: device.ID, Sequence: 2, PowerWatts: 10, MeasuredAt: time.Now().UTC()}); !errors.Is(err, model.ErrConflict) {
		t.Fatalf("disabled device telemetry err=%v", err)
	}
	if err := r.SetPlanState(ctx, plan.ID, model.PlanScheduled, model.PlanRunning); !errors.Is(err, model.ErrConflict) {
		t.Fatalf("plan started with disabled device: %v", err)
	}
	blockedRun, err := r.QueueRun(ctx, automation.ID, "disabled-device")
	if err != nil {
		t.Fatal(err)
	}
	claimedBlockedRun, err := r.ClaimRun(ctx)
	if err != nil || claimedBlockedRun.ID != blockedRun.ID {
		t.Fatalf("claimed run=%+v err=%v", claimedBlockedRun, err)
	}
	if err := r.ExecuteAutomationRun(ctx, blockedRun.ID, time.Now().UTC()); !errors.Is(err, model.ErrConflict) {
		t.Fatalf("automation executed disabled device: %v", err)
	}
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_messages WHERE payload->>'run_id'=$1`, fmt.Sprint(blockedRun.ID)).Scan(&count); err != nil || count != 0 {
		t.Fatalf("blocked automation outbox count=%d err=%v", count, err)
	}
	if err := r.RequeueRun(ctx, blockedRun.ID); err != nil {
		t.Fatal(err)
	}
	if err := r.TransitionDevice(ctx, device.ID, model.DeviceDisabled, model.DeviceEnabled, device.Version+3); err != nil {
		t.Fatal(err)
	}
	if err := r.SetPlanState(ctx, plan.ID, model.PlanScheduled, model.PlanRunning); err != nil {
		t.Fatal(err)
	}
	reclaimedRun, err := r.ClaimRun(ctx)
	if err != nil || reclaimedRun.ID != blockedRun.ID {
		t.Fatalf("reclaimed run=%+v err=%v", reclaimedRun, err)
	}
	if err := r.ExecuteAutomationRun(ctx, reclaimedRun.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `UPDATE outbox_messages SET delivered_at=now() WHERE topic='device.command'`); err != nil {
		t.Fatal(err)
	}
	if err := repo.RollbackProof(ctx, r, household.ID); err == nil {
		t.Fatal("rollback proof unexpectedly committed")
	}
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM households WHERE name='rollback-proof'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("rollback count=%d err=%v", count, err)
	}
	if err := r.AddAudit(ctx, model.AuditEvent{HouseholdID: household.ID, ActorMemberID: &member.ID, RequestID: "req-1", ObjectType: "device", ObjectID: "1", Action: "created", Payload: []byte(`{}`)}); err != nil {
		t.Fatal(err)
	}
	if err := r.AddOutbox(ctx, model.OutboxMessage{HouseholdID: household.ID, Topic: "device.created", Payload: []byte(`{"id":1}`), AvailableAt: time.Now().UTC().Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	claimed, err := r.ClaimOutbox(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.ClaimOutbox(ctx); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("leased outbox message was claimed twice: %v", err)
	}
	if err := r.MarkOutbox(ctx, claimed.ID, true, time.Time{}); err != nil {
		t.Fatal(err)
	}
	if err := r.AddOutbox(ctx, model.OutboxMessage{HouseholdID: household.ID, Topic: "device.failed", Payload: []byte(`{"id":2}`), AvailableAt: time.Now().UTC().Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	failed, err := r.ClaimOutbox(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.MarkOutboxFailed(ctx, failed.ID, "webhook rejected delivery"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ClaimOutbox(ctx); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("permanently failed outbox message was reclaimed: %v", err)
	}
	var failedAt *time.Time
	var failureReason *string
	if err := database.SQL.QueryRowContext(ctx, `SELECT failed_at,failure_reason FROM outbox_messages WHERE id=$1`, failed.ID).Scan(&failedAt, &failureReason); err != nil || failedAt == nil || failureReason == nil || *failureReason != "webhook rejected delivery" {
		t.Fatalf("failed_at=%v failure_reason=%v err=%v", failedAt, failureReason, err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	database, err = db.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := db.Migrate(ctx, database); err != nil {
		t.Fatal(err)
	}
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM devices WHERE external_id='lamp'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("restart persistence count=%d err=%v", count, err)
	}
}
