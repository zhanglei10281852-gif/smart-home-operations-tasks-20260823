package repo_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/zhanglei10281852-gif/smart-home-operations/internal/db"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/repo"
)

func TestAutomationDispatchAuditFailureIsAtomic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
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
	if _, err := database.SQL.ExecContext(ctx, `TRUNCATE audit_events,outbox_messages,automation_runs,automation_actions,automations,plan_devices,energy_plans,telemetry,device_capabilities,devices,sessions,members,households RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `DROP TRIGGER IF EXISTS reject_automation_dispatch_audit ON audit_events; DROP FUNCTION IF EXISTS reject_automation_dispatch_audit()`); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = database.SQL.ExecContext(context.Background(), `DROP TRIGGER IF EXISTS reject_automation_dispatch_audit ON audit_events; DROP FUNCTION IF EXISTS reject_automation_dispatch_audit()`)
	}()
	r := repo.New(database)
	household, err := r.CreateHousehold(ctx, "Dispatch atomic household", "UTC", 22000)
	if err != nil {
		t.Fatal(err)
	}
	var deviceID int64
	if err := database.SQL.QueryRowContext(ctx, `INSERT INTO devices(household_id,external_id,kind,state,firmware) VALUES($1,'dispatch-lamp','light','enabled','1.7') RETURNING id`, household.ID).Scan(&deviceID); err != nil {
		t.Fatal(err)
	}
	automation, err := r.CreateAutomation(ctx, model.Automation{HouseholdID: household.ID, Name: "arrival dispatch", TriggerKind: "presence"}, []model.AutomationAction{{DeviceID: deviceID, Action: "on", Ordinal: 0}})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.SetAutomationState(ctx, automation.ID, model.AutomationDraft, model.AutomationActive); err != nil {
		t.Fatal(err)
	}
	if _, err := r.QueueRun(ctx, automation.ID, "dispatch-audit-failure"); err != nil {
		t.Fatal(err)
	}
	run, err := r.ClaimRun(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `CREATE OR REPLACE FUNCTION reject_automation_dispatch_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.object_type='automation_run' AND NEW.action='dispatched' THEN RAISE EXCEPTION 'automation audit unavailable'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_automation_dispatch_audit BEFORE INSERT ON audit_events FOR EACH ROW EXECUTE FUNCTION reject_automation_dispatch_audit()`); err != nil {
		t.Fatal(err)
	}
	if err := r.ExecuteAutomationRun(ctx, run.ID, time.Now().UTC()); err == nil {
		t.Fatal("automation dispatch unexpectedly succeeded while its audit was rejected")
	}
	var state string
	var outbox, audits int
	if err := database.SQL.QueryRowContext(ctx, `SELECT state FROM automation_runs WHERE id=$1`, run.ID).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_messages WHERE payload->>'run_id'=$1`, stringID(run.ID)).Scan(&outbox); err != nil {
		t.Fatal(err)
	}
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE object_type='automation_run' AND object_id=$1`, stringID(run.ID)).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if state != "running" || outbox != 0 || audits != 0 {
		t.Fatalf("failed dispatch left contradictory state: state=%s outbox=%d audits=%d", state, outbox, audits)
	}
	if _, err := database.SQL.ExecContext(ctx, `DROP TRIGGER reject_automation_dispatch_audit ON audit_events; DROP FUNCTION reject_automation_dispatch_audit()`); err != nil {
		t.Fatal(err)
	}
	if err := r.ExecuteAutomationRun(ctx, run.ID, time.Now().UTC()); err != nil {
		t.Fatalf("valid dispatch failed: %v", err)
	}
	if err := database.SQL.QueryRowContext(ctx, `SELECT state FROM automation_runs WHERE id=$1`, run.ID).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_messages WHERE payload->>'run_id'=$1`, stringID(run.ID)).Scan(&outbox); err != nil {
		t.Fatal(err)
	}
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE object_type='automation_run' AND object_id=$1`, stringID(run.ID)).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if state != "succeeded" || outbox != 1 || audits != 1 {
		t.Fatalf("valid dispatch is incomplete: state=%s outbox=%d audits=%d", state, outbox, audits)
	}
}

func stringID(value int64) string {
	return fmt.Sprint(value)
}
