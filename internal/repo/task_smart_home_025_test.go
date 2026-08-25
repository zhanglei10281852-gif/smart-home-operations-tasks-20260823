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

func TestAutomationCreationFailureRollsBackDefinition(t *testing.T) {
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
	household, err := r.CreateHousehold(ctx, "Automation rollback household", "UTC", 30000)
	if err != nil {
		t.Fatal(err)
	}
	var lampID, thermostatID int64
	if err := database.SQL.QueryRowContext(ctx, `INSERT INTO devices(household_id,external_id,kind,state,firmware) VALUES($1,'hall-lamp','light','enabled','1.2') RETURNING id`, household.ID).Scan(&lampID); err != nil {
		t.Fatal(err)
	}
	if err := database.SQL.QueryRowContext(ctx, `INSERT INTO devices(household_id,external_id,kind,state,firmware) VALUES($1,'hall-thermostat','thermostat','enabled','2.4') RETURNING id`, household.ID).Scan(&thermostatID); err != nil {
		t.Fatal(err)
	}
	_, err = r.CreateAutomation(ctx, model.Automation{HouseholdID: household.ID, Name: "broken arrival", TriggerKind: "presence"}, []model.AutomationAction{{DeviceID: lampID, Action: "on", Ordinal: 0}, {DeviceID: thermostatID + 10000, Action: "comfort", Ordinal: 1}})
	if err == nil {
		t.Fatal("automation creation unexpectedly succeeded with an unavailable trailing device")
	}
	var definitions, actions int
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM automations WHERE household_id=$1`, household.ID).Scan(&definitions); err != nil {
		t.Fatal(err)
	}
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM automation_actions`).Scan(&actions); err != nil {
		t.Fatal(err)
	}
	if definitions != 0 || actions != 0 {
		t.Fatalf("failed automation left partial rows: definitions=%d actions=%d", definitions, actions)
	}
	created, err := r.CreateAutomation(ctx, model.Automation{HouseholdID: household.ID, Name: "valid arrival", TriggerKind: "presence"}, []model.AutomationAction{{DeviceID: lampID, Action: "on", Ordinal: 0}, {DeviceID: thermostatID, Action: "comfort", Ordinal: 1}})
	if err != nil {
		t.Fatalf("valid automation creation failed: %v", err)
	}
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM automation_actions WHERE automation_id=$1`, created.ID).Scan(&actions); err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 || actions != 2 {
		t.Fatalf("valid automation is incomplete: automation=%d actions=%d", created.ID, actions)
	}
}
