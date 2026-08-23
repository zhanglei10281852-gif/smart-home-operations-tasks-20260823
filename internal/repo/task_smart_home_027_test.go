package repo_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/zhanglei10281852-gif/smart-home-operations/internal/db"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/repo"
)

func TestDispatchAuditAndOutboxCommitTogether(t *testing.T) {
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
	if _, err := database.SQL.ExecContext(ctx, `DROP TRIGGER IF EXISTS reject_dispatch_outbox ON outbox_messages; DROP FUNCTION IF EXISTS reject_dispatch_outbox()`); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = database.SQL.ExecContext(context.Background(), `DROP TRIGGER IF EXISTS reject_dispatch_outbox ON outbox_messages; DROP FUNCTION IF EXISTS reject_dispatch_outbox()`)
	}()
	r := repo.New(database)
	household, err := r.CreateHousehold(ctx, "Dispatch persistence household", "UTC", 18000)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `CREATE OR REPLACE FUNCTION reject_dispatch_outbox() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.topic='device.dispatch' THEN RAISE EXCEPTION 'outbox unavailable'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_dispatch_outbox BEFORE INSERT ON outbox_messages FOR EACH ROW EXECUTE FUNCTION reject_dispatch_outbox()`); err != nil {
		t.Fatal(err)
	}
	audit := model.AuditEvent{HouseholdID: household.ID, RequestID: "dispatch-failure", ObjectType: "device", ObjectID: "42", Action: "dispatched", Payload: json.RawMessage(`{"action":"on"}`)}
	message := model.OutboxMessage{HouseholdID: household.ID, Topic: "device.dispatch", Payload: json.RawMessage(`{"device_id":42}`), AvailableAt: time.Now().UTC()}
	if err := r.AuditAndMessage(ctx, audit, message); err == nil {
		t.Fatal("dispatch persistence unexpectedly succeeded while outbox was rejected")
	}
	var audits, messages int
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE request_id='dispatch-failure'`).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_messages WHERE topic='device.dispatch'`).Scan(&messages); err != nil {
		t.Fatal(err)
	}
	if audits != 0 || messages != 0 {
		t.Fatalf("failed dispatch left partial persistence: audits=%d messages=%d", audits, messages)
	}
	if _, err := database.SQL.ExecContext(ctx, `DROP TRIGGER reject_dispatch_outbox ON outbox_messages; DROP FUNCTION reject_dispatch_outbox()`); err != nil {
		t.Fatal(err)
	}
	audit.RequestID = "dispatch-success"
	if err := r.AuditAndMessage(ctx, audit, message); err != nil {
		t.Fatalf("valid dispatch persistence failed: %v", err)
	}
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE request_id='dispatch-success'`).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_messages WHERE topic='device.dispatch'`).Scan(&messages); err != nil {
		t.Fatal(err)
	}
	if audits != 1 || messages != 1 {
		t.Fatalf("valid dispatch persistence is incomplete: audits=%d messages=%d", audits, messages)
	}
}
