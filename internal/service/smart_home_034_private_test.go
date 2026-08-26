package service_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/zhanglei10281852-gif/smart-home-operations/internal/db"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/repo"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/service"
)

func TestEnergyPlanCannotStartAfterLinkedDeviceBecomesIneligible(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	database := openTaskDatabase034(t, ctx)
	defer database.Close()
	r := repo.New(database)
	now := time.Now().UTC()
	household, err := r.CreateHousehold(ctx, "energy-race-034", "UTC", 100)
	if err != nil {
		t.Fatal(err)
	}
	device, err := r.CreateDevice(ctx, model.Device{HouseholdID: household.ID, ExternalID: "meter-034", Kind: model.KindMeter, Firmware: "1.0"}, []string{"power"})
	if err != nil {
		t.Fatal(err)
	}
	if err = r.TransitionDevice(ctx, device.ID, model.DevicePending, model.DevicePaired, device.Version); err != nil {
		t.Fatal(err)
	}
	if err = r.TransitionDevice(ctx, device.ID, model.DevicePaired, model.DeviceEnabled, device.Version+1); err != nil {
		t.Fatal(err)
	}
	plan, err := r.CreatePlan(ctx, model.EnergyPlan{HouseholdID: household.ID, Name: "peak-window", BudgetCents: 100, StartsAt: now.Add(-time.Minute), EndsAt: now.Add(time.Hour)}, []model.PlanDevice{{DeviceID: device.ID, TargetWatts: 20}})
	if err != nil {
		t.Fatal(err)
	}
	if err = r.SetPlanState(ctx, plan.ID, model.PlanDraft, model.PlanScheduled); err != nil {
		t.Fatal(err)
	}

	const lockID int64 = 34034
	locker, err := database.SQL.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer locker.Close()
	if _, err = locker.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, lockID); err != nil {
		t.Fatal(err)
	}
	defer locker.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, lockID)
	if _, err = database.SQL.ExecContext(ctx, fmt.Sprintf(`
		CREATE OR REPLACE FUNCTION task_034_block_plan_start() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN PERFORM pg_advisory_xact_lock(%d); RETURN NEW; END $$;
		CREATE TRIGGER task_034_block_plan_start BEFORE UPDATE ON energy_plans
		FOR EACH ROW WHEN (NEW.state='running') EXECUTE FUNCTION task_034_block_plan_start()`, lockID)); err != nil {
		t.Fatal(err)
	}
	defer database.SQL.ExecContext(context.Background(), `DROP TRIGGER IF EXISTS task_034_block_plan_start ON energy_plans; DROP FUNCTION IF EXISTS task_034_block_plan_start()`)

	energy := service.NewEnergy(r, service.FixedClock{Value: now})
	started := make(chan error, 1)
	go func() { started <- energy.Start(ctx, plan.ID) }()
	waitForBlockedQuery034(t, ctx, database.SQL, "UPDATE energy_plans")
	if err = r.TransitionDevice(ctx, device.ID, model.DeviceEnabled, model.DeviceDisabled, device.Version+2); err != nil {
		t.Fatal(err)
	}
	if _, err = locker.ExecContext(ctx, `SELECT pg_advisory_unlock($1)`, lockID); err != nil {
		t.Fatal(err)
	}
	if err = <-started; !errors.Is(err, model.ErrConflict) {
		t.Fatalf("plan start was accepted after linked device disable: %v", err)
	}
	persisted, err := r.GetPlan(ctx, plan.ID)
	if err != nil || persisted.State != model.PlanScheduled {
		t.Fatalf("plan state=%s err=%v", persisted.State, err)
	}
}

func openTaskDatabase034(t *testing.T, ctx context.Context) *db.DB {
	t.Helper()
	database, err := db.Open(ctx, "postgres://smart_home:smart_home@127.0.0.1:55432/smart_home?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Migrate(ctx, database); err != nil {
		database.Close()
		t.Fatal(err)
	}
	if _, err = database.SQL.ExecContext(ctx, `TRUNCATE audit_events,outbox_messages,automation_runs,automation_actions,automations,plan_devices,energy_plans,telemetry,device_capabilities,devices,sessions,members,households RESTART IDENTITY CASCADE`); err != nil {
		database.Close()
		t.Fatal(err)
	}
	return database
}

func waitForBlockedQuery034(t *testing.T, ctx context.Context, database *sql.DB, fragment string) {
	t.Helper()
	for {
		var count int
		if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event='advisory' AND position($1 in query) > 0`, fragment).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count >= 1 {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}
}
