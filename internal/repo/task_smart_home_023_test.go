package repo_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/zhanglei10281852-gif/smart-home-operations/internal/db"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/repo"
)

func TestEnrollmentFailureLeavesNoPartialDevice(t *testing.T) {
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
	r := repo.New(database)
	household, err := r.CreateHousehold(ctx, "Enrollment rollback family", "UTC", 9000)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.SQL.ExecContext(ctx, `CREATE OR REPLACE FUNCTION reject_scene_capability() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.capability='scene' THEN RAISE EXCEPTION 'scene capability blocked'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_scene_capability BEFORE INSERT ON device_capabilities FOR EACH ROW EXECUTE FUNCTION reject_scene_capability()`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = database.SQL.ExecContext(context.Background(), `DROP TRIGGER IF EXISTS reject_scene_capability ON device_capabilities; DROP FUNCTION IF EXISTS reject_scene_capability()`)
	})

	_, err = r.CreateDevice(ctx, model.Device{HouseholdID: household.ID, ExternalID: "entry-panel", Kind: model.KindController, Firmware: "4.3.1"}, []string{"power", "scene", "automation"})
	if err == nil {
		t.Fatal("enrollment unexpectedly succeeded after a capability was rejected")
	}
	var devices, capabilities int
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM devices WHERE household_id=$1 AND external_id='entry-panel'`, household.ID).Scan(&devices); err != nil {
		t.Fatal(err)
	}
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM device_capabilities WHERE capability='power'`).Scan(&capabilities); err != nil {
		t.Fatal(err)
	}
	if devices != 0 || capabilities != 0 {
		t.Fatalf("failed enrollment left partial state: devices=%d capabilities=%d", devices, capabilities)
	}
	if _, err := database.SQL.ExecContext(ctx, `DROP TRIGGER reject_scene_capability ON device_capabilities; DROP FUNCTION reject_scene_capability()`); err != nil {
		t.Fatal(err)
	}
	created, err := r.CreateDevice(ctx, model.Device{HouseholdID: household.ID, ExternalID: "living-room-panel", Kind: model.KindController, Firmware: "4.3.1"}, []string{"power", "automation"})
	if err != nil {
		t.Fatalf("valid multi-capability enrollment failed: %v", err)
	}
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM device_capabilities WHERE device_id=$1`, created.ID).Scan(&capabilities); err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 || capabilities != 2 {
		t.Fatalf("valid enrollment is incomplete: device=%d capabilities=%d", created.ID, capabilities)
	}
}
