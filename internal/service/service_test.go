package service

import (
	"context"
	"errors"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/domain"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"testing"
	"time"
)

type fakeRepo struct {
	member        model.Member
	hash          string
	session       model.Session
	sessionErr    error
	device        model.Device
	deviceErr     error
	transitionErr error
	telemetry     []model.Telemetry
	plan          model.EnergyPlan
	automation    model.Automation
	run           model.AutomationRun
	finishErr     error
}

func (f *fakeRepo) AddMember(context.Context, int64, string, string, model.Role) (model.Member, error) {
	return f.member, nil
}
func (f *fakeRepo) FindMember(context.Context, int64, string) (model.Member, string, error) {
	return f.member, f.hash, nil
}
func (f *fakeRepo) CreateSession(_ context.Context, s model.Session) error {
	f.session = s
	return f.sessionErr
}
func (f *fakeRepo) GetSession(_ context.Context, _ string) (model.Session, error) {
	return f.session, f.sessionErr
}
func (f *fakeRepo) RevokeSession(context.Context, string, time.Time) error  { return nil }
func (f *fakeRepo) MemberByID(context.Context, int64) (model.Member, error) { return f.member, nil }
func (f *fakeRepo) CreateHousehold(context.Context, string, string, int64) (model.Household, error) {
	return model.Household{ID: 1}, nil
}
func (f *fakeRepo) CreateHouseholdWithOwner(context.Context, string, string, int64, string, string) (model.Household, model.Member, error) {
	return model.Household{ID: 1}, f.member, nil
}
func (f *fakeRepo) CreateDevice(context.Context, model.Device, []string) (model.Device, error) {
	return f.device, nil
}
func (f *fakeRepo) GetDevice(context.Context, int64) (model.Device, error) {
	return f.device, f.deviceErr
}
func (f *fakeRepo) TransitionDevice(context.Context, int64, model.DeviceState, model.DeviceState, int64) error {
	return f.transitionErr
}
func (f *fakeRepo) TouchDevice(context.Context, int64, time.Time) error    { return nil }
func (f *fakeRepo) InsertTelemetry(context.Context, model.Telemetry) error { return nil }
func (f *fakeRepo) TelemetryWindow(context.Context, int64, time.Time, time.Time) ([]model.Telemetry, error) {
	return f.telemetry, nil
}
func (f *fakeRepo) CreatePlan(_ context.Context, plan model.EnergyPlan, _ []model.PlanDevice) (model.EnergyPlan, error) {
	f.plan = plan
	f.plan.ID = 7
	f.plan.State = model.PlanDraft
	return f.plan, nil
}
func (f *fakeRepo) GetPlan(context.Context, int64) (model.EnergyPlan, error) { return f.plan, nil }
func (f *fakeRepo) SetPlanState(_ context.Context, _ int64, from, to model.PlanState) error {
	if f.plan.State != from {
		return model.ErrConflict
	}
	f.plan.State = to
	return nil
}
func (f *fakeRepo) CreateAutomation(_ context.Context, automation model.Automation, _ []model.AutomationAction) (model.Automation, error) {
	f.automation = automation
	f.automation.ID = 8
	f.automation.State = model.AutomationDraft
	return f.automation, nil
}
func (f *fakeRepo) GetAutomation(context.Context, int64) (model.Automation, error) {
	return f.automation, nil
}
func (f *fakeRepo) SetAutomationState(_ context.Context, _ int64, from, to model.AutomationState) error {
	if f.automation.State != from {
		return model.ErrConflict
	}
	f.automation.State = to
	return nil
}
func (f *fakeRepo) QueueRun(context.Context, int64, string) (model.AutomationRun, error) {
	return f.run, nil
}
func (f *fakeRepo) FinishRun(context.Context, int64, model.RunState, string, time.Time) error {
	return f.finishErr
}
func (f *fakeRepo) ExecuteAutomationRun(context.Context, int64, time.Time) error { return f.finishErr }
func (f *fakeRepo) AddAudit(context.Context, model.AuditEvent) error             { return nil }
func (f *fakeRepo) AddOutbox(context.Context, model.OutboxMessage) error         { return nil }

func TestAuthLoginAndLogout(t *testing.T) {
	clock := FixedClock{Value: time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)}
	hash, err := hashPassword("correct-password")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeRepo{member: model.Member{ID: 3, HouseholdID: 1, Email: "user@example.com", Role: model.RoleOperator, Active: true}, hash: hash}
	a := NewAuth(nil, clock)
	a.Repo = f
	s, m, err := a.Login(context.Background(), 1, "user@example.com", "correct-password")
	if err != nil {
		t.Fatal(err)
	}
	if s.ID == "" || m.ID != 3 || !s.ExpiresAt.Equal(clock.Value.Add(a.SessionTTL)) {
		t.Fatalf("session=%+v member=%+v", s, m)
	}
	f.hash, err = hashPassword("different")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Login(context.Background(), 1, "user@example.com", "correct-password"); !errors.Is(err, model.ErrForbidden) {
		t.Fatalf("wrong password err=%v", err)
	}
	if err := a.Logout(context.Background(), s.ID); err != nil {
		t.Fatal(err)
	}
}
func TestPasswordHashUsesSaltAndRejectsMalformedValues(t *testing.T) {
	first, err := hashPassword("correct-password")
	if err != nil {
		t.Fatal(err)
	}
	second, err := hashPassword("correct-password")
	if err != nil {
		t.Fatal(err)
	}
	if first == second || !passwordMatches(first, "correct-password") || passwordMatches(first, "wrong-password") {
		t.Fatalf("password hashes were not salted or verified correctly")
	}
	for _, malformed := range []string{"", "pbkdf2-sha256$1$bad$bad", "sha256$210000$bad$bad"} {
		if passwordMatches(malformed, "correct-password") {
			t.Fatalf("malformed hash accepted: %q", malformed)
		}
	}
}
func TestAuthRejectsInvalidRegistration(t *testing.T) {
	a := NewAuth(&fakeRepo{}, model.RealClock{})
	for _, tc := range []struct {
		email, password string
		household       int64
	}{{"", "short", 1}, {"a@b.com", "short", 1}, {"a@b.com", "long-enough-password", 0}} {
		if _, err := a.Register(context.Background(), tc.household, tc.email, tc.password, model.RoleViewer); err == nil {
			t.Fatalf("accepted invalid %#v", tc)
		}
	}
}
func TestAuthAuthorization(t *testing.T) {
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	f := &fakeRepo{member: model.Member{ID: 7, Role: model.RoleOperator, Active: true}, session: model.Session{ID: "s", MemberID: 7, ExpiresAt: now.Add(time.Hour)}}
	a := NewAuth(f, FixedClock{Value: now})
	if _, err := a.Authorize(context.Background(), "s", model.RoleOperator); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Authorize(context.Background(), "s", model.RoleOwner); err == nil {
		t.Fatal("operator received owner permission")
	}
	f.member.Active = false
	if _, err := a.Authorize(context.Background(), "s", model.RoleViewer); err == nil {
		t.Fatal("inactive member authorized")
	}
	f.member.Active = true
	f.session.ExpiresAt = now
	if _, err := a.Authorize(context.Background(), "s", model.RoleViewer); !errors.Is(err, model.ErrForbidden) {
		t.Fatalf("expired session err=%v", err)
	}
	f.session.ExpiresAt = now.Add(time.Hour)
	revoked := now.Add(-time.Minute)
	f.session.RevokedAt = &revoked
	if _, err := a.Authorize(context.Background(), "s", model.RoleViewer); !errors.Is(err, model.ErrForbidden) {
		t.Fatalf("revoked session err=%v", err)
	}
}

func TestDeviceLifecycle(t *testing.T) {
	clock := FixedClock{Value: time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)}
	f := &fakeRepo{device: model.Device{ID: 4, HouseholdID: 1, ExternalID: "lamp", Kind: model.KindLight, State: model.DevicePending, Version: 1, CreatedAt: clock.Value}}
	s := NewDevices(f, clock)
	d, err := s.Enroll(context.Background(), 1, "lamp", model.KindLight, "1.0", []string{"power"})
	if err != nil || d.ID != 4 {
		t.Fatalf("enroll d=%+v err=%v", d, err)
	}
	if err := s.Pair(context.Background(), 4); err != nil {
		t.Fatal(err)
	}
	f.device.State = model.DevicePaired
	if err := s.Enable(context.Background(), 4); err != nil {
		t.Fatal(err)
	}
	f.device.State = model.DeviceEnabled
	if err := s.Disable(context.Background(), 4); err != nil {
		t.Fatal(err)
	}
	f.device.State = model.DeviceDisabled
	if err := s.Enable(context.Background(), 4); err != nil {
		t.Fatal(err)
	}
	if !DeviceFresh(model.Device{LastSeenAt: ptrTime(clock.Value.Add(-time.Minute))}, clock.Value, 2*time.Minute) {
		t.Fatal("fresh reading marked stale")
	}
	if DeviceFresh(model.Device{LastSeenAt: ptrTime(clock.Value.Add(-3 * time.Minute))}, clock.Value, 2*time.Minute) {
		t.Fatal("stale reading marked fresh")
	}
}
func TestDeviceLifecycleErrors(t *testing.T) {
	f := &fakeRepo{device: model.Device{ID: 4, State: model.DeviceRetired, Version: 1}}
	s := NewDevices(f, model.RealClock{})
	if err := s.Enable(context.Background(), 4); err == nil {
		t.Fatal("retired device enabled")
	}
	if _, err := s.Enroll(context.Background(), 1, "", model.KindLight, "1", nil); err == nil {
		t.Fatal("empty external accepted")
	}
	if _, err := s.Enroll(context.Background(), 1, "x", model.DeviceKind("unknown"), "1", nil); err == nil {
		t.Fatal("unknown kind accepted")
	}
}

func TestTelemetryService(t *testing.T) {
	clock := FixedClock{Value: time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)}
	f := &fakeRepo{}
	s := NewTelemetry(f, clock)
	for _, tc := range []model.Telemetry{{DeviceID: 1, Sequence: 1, PowerWatts: 5, MeasuredAt: clock.Value}, {DeviceID: 1, Sequence: 2, PowerWatts: 7, MeasuredAt: clock.Value.Add(time.Minute)}} {
		if err := s.Record(context.Background(), tc); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Record(context.Background(), model.Telemetry{DeviceID: 1, Sequence: 3, PowerWatts: -1, MeasuredAt: clock.Value}); err == nil {
		t.Fatal("negative power accepted")
	}
	if err := s.Record(context.Background(), model.Telemetry{DeviceID: 1, Sequence: 3, PowerWatts: 1, MeasuredAt: clock.Value.Add(10 * time.Minute)}); err == nil {
		t.Fatal("future telemetry accepted")
	}
	if _, err := s.Window(context.Background(), 1, clock.Value, clock.Value.Add(8*24*time.Hour)); err == nil {
		t.Fatal("long telemetry window accepted")
	}
	if _, err := s.Window(context.Background(), 1, clock.Value.Add(time.Hour), clock.Value); err == nil {
		t.Fatal("reversed telemetry window accepted")
	}
}
func TestTelemetryAverage(t *testing.T) {
	rows := []model.Telemetry{{PowerWatts: 2}, {PowerWatts: 4}, {PowerWatts: 8}}
	if got := AveragePower(rows); got != 14.0/3.0 {
		t.Fatalf("avg=%v", got)
	}
	if AveragePower(nil) != 0 {
		t.Fatal("empty average nonzero")
	}
}

func TestEnergyService(t *testing.T) {
	clock := &FixedClock{Value: time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)}
	f := &fakeRepo{}
	s := NewEnergy(f, clock)
	p, err := s.Draft(context.Background(), model.EnergyPlan{HouseholdID: 1, Name: "evening", BudgetCents: 100, StartsAt: clock.Value.Add(time.Hour), EndsAt: clock.Value.Add(2 * time.Hour)}, []model.PlanDevice{{DeviceID: 1, TargetWatts: 20}})
	if err != nil || p.ID != 7 {
		t.Fatalf("plan=%+v err=%v", p, err)
	}
	if err := s.Schedule(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
	clock.Value = clock.Value.Add(time.Hour)
	if err := s.Start(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
	if err := s.Complete(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
	for _, p := range []model.EnergyPlan{{HouseholdID: 1, Name: "", BudgetCents: 1, StartsAt: clock.Value, EndsAt: clock.Value.Add(time.Hour)}, {HouseholdID: 1, Name: "bad", BudgetCents: 1, StartsAt: clock.Value.Add(time.Hour), EndsAt: clock.Value.Add(33 * 24 * time.Hour)}} {
		if _, err := s.Draft(context.Background(), p, []model.PlanDevice{{DeviceID: 1, TargetWatts: 1}}); err == nil {
			t.Fatalf("invalid plan accepted %+v", p)
		}
	}
}

func TestAutomationService(t *testing.T) {
	f := &fakeRepo{device: model.Device{ID: 1, HouseholdID: 1, Kind: model.KindLight, State: model.DeviceEnabled}, run: model.AutomationRun{ID: 9, AutomationID: 8, State: model.RunQueued}}
	a := NewAutomation(f, model.RealClock{})
	created, err := a.Create(context.Background(), model.Automation{HouseholdID: 1, Name: "arrive", TriggerKind: "presence"}, []model.AutomationAction{{DeviceID: 1, Action: "on", Ordinal: 0}})
	if err != nil || created.ID != 8 {
		t.Fatalf("created=%+v err=%v", created, err)
	}
	if err := a.Activate(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}
	run, err := a.Queue(context.Background(), created.ID, "key-1")
	if err != nil || run.ID != 9 {
		t.Fatalf("run=%+v err=%v", run, err)
	}
	if err := a.Execute(context.Background(), 9); err != nil {
		t.Fatal(err)
	}
	if err := a.Execute(context.Background(), 0); err == nil {
		t.Fatal("zero run accepted")
	}
	if err := a.ValidateAction(model.KindThermostat, "on"); err == nil {
		t.Fatal("light action accepted by thermostat")
	}
	if err := a.ValidateAction(model.KindLight, "on"); err != nil {
		t.Fatal(err)
	}
}

func TestRetryPolicy(t *testing.T) {
	p := RetryPolicy{Base: 100 * time.Millisecond}
	if p.Delay(1) != 100*time.Millisecond || p.Delay(2) != 200*time.Millisecond || p.Delay(3) != 400*time.Millisecond {
		t.Fatal("retry delays incorrect")
	}
	if p.Delay(20) <= p.Delay(3) {
		t.Fatal("retry delay did not grow")
	}
}
func TestAuditAndNotification(t *testing.T) {
	f := &fakeRepo{}
	a := NewAudit(f)
	if err := a.Record(context.Background(), 1, nil, "req", "device", "1", "enabled", map[string]any{"source": "test"}); err != nil {
		t.Fatal(err)
	}
	n := &collectNotifier{}
	s := NewNotifications(n)
	if err := s.SendAlert(context.Background(), "member", "overheat", "critical"); err != nil {
		t.Fatal(err)
	}
	if len(n.messages) != 1 {
		t.Fatal("notification missing")
	}
}

type collectNotifier struct{ messages []domain.Notification }

func (n *collectNotifier) Send(_ context.Context, m domain.Notification) error {
	n.messages = append(n.messages, m)
	return nil
}
func ptrTime(t time.Time) *time.Time { return &t }
