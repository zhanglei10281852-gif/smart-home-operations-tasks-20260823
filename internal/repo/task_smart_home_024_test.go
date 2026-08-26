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

func TestPlanDraftFailureRollsBackAllRows(t *testing.T) {
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
	household, err := r.CreateHousehold(ctx, "Plan rollback household", "UTC", 45000)
	if err != nil {
		t.Fatal(err)
	}
	var firstDevice, secondDevice int64
	if err := database.SQL.QueryRowContext(ctx, `INSERT INTO devices(household_id,external_id,kind,state,firmware) VALUES($1,'heater','thermostat','enabled','2.0') RETURNING id`, household.ID).Scan(&firstDevice); err != nil {
		t.Fatal(err)
	}
	if err := database.SQL.QueryRowContext(ctx, `INSERT INTO devices(household_id,external_id,kind,state,firmware) VALUES($1,'battery','meter','enabled','3.0') RETURNING id`, household.ID).Scan(&secondDevice); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	_, err = r.CreatePlan(ctx, model.EnergyPlan{HouseholdID: household.ID, Name: "broken evening plan", BudgetCents: 1000, StartsAt: now.Add(time.Hour), EndsAt: now.Add(2 * time.Hour)}, []model.PlanDevice{{DeviceID: firstDevice, TargetWatts: 800}, {DeviceID: secondDevice + 10000, TargetWatts: 300}})
	if err == nil {
		t.Fatal("plan creation unexpectedly succeeded with an ineligible trailing device")
	}
	var plans, links int
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM energy_plans WHERE household_id=$1`, household.ID).Scan(&plans); err != nil {
		t.Fatal(err)
	}
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM plan_devices`).Scan(&links); err != nil {
		t.Fatal(err)
	}
	if plans != 0 || links != 0 {
		t.Fatalf("failed plan draft left partial rows: plans=%d links=%d", plans, links)
	}
	created, err := r.CreatePlan(ctx, model.EnergyPlan{HouseholdID: household.ID, Name: "valid evening plan", BudgetCents: 1200, StartsAt: now.Add(3 * time.Hour), EndsAt: now.Add(4 * time.Hour)}, []model.PlanDevice{{DeviceID: firstDevice, TargetWatts: 700}, {DeviceID: secondDevice, TargetWatts: 250}})
	if err != nil {
		t.Fatalf("valid plan draft failed: %v", err)
	}
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM plan_devices WHERE plan_id=$1`, created.ID).Scan(&links); err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 || links != 2 {
		t.Fatalf("valid plan draft is incomplete: plan=%d links=%d", created.ID, links)
	}
}
