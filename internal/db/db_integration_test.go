package db_test

import (
	"context"
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
		dsn = "postgres://smart_home:smart_home@localhost:55432/smart_home?sslmode=disable"
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
	versions, err := db.MigrationVersions(ctx, database)
	if err != nil || len(versions) == 0 {
		t.Fatalf("versions=%v err=%v", versions, err)
	}
	_, err = database.SQL.ExecContext(ctx, `TRUNCATE audit_events,outbox_messages,automation_runs,automation_actions,automations,plan_devices,energy_plans,telemetry,device_capabilities,devices,sessions,members,households RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatal(err)
	}
	r := repo.New(database)
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
	run, err := r.QueueRun(ctx, automation.ID, "idempotent-key")
	if err != nil {
		t.Fatal(err)
	}
	same, err := r.QueueRun(ctx, automation.ID, "idempotent-key")
	if err != nil || same.ID != run.ID {
		t.Fatalf("idempotency run=%+v same=%+v err=%v", run, same, err)
	}
	if err := repo.RollbackProof(ctx, r, household.ID); err == nil {
		t.Fatal("rollback proof unexpectedly committed")
	}
	var count int
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM households WHERE name='rollback-proof'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("rollback count=%d err=%v", count, err)
	}
	if err := r.AddAudit(ctx, model.AuditEvent{HouseholdID: household.ID, ActorMemberID: &member.ID, RequestID: "req-1", ObjectType: "device", ObjectID: "1", Action: "created", Payload: []byte(`{}`)}); err != nil {
		t.Fatal(err)
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
