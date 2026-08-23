package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/service"
)

type authHTTPRepo struct {
	member  model.Member
	session model.Session
}

func (r *authHTTPRepo) AddMember(context.Context, int64, string, string, model.Role) (model.Member, error) {
	return r.member, nil
}
func (r *authHTTPRepo) FindMember(context.Context, int64, string) (model.Member, string, error) {
	return model.Member{}, "", model.ErrNotFound
}
func (r *authHTTPRepo) CreateSession(context.Context, model.Session) error { return nil }
func (r *authHTTPRepo) GetSession(context.Context, string) (model.Session, error) {
	return r.session, nil
}
func (r *authHTTPRepo) MemberByID(context.Context, int64) (model.Member, error) {
	return r.member, nil
}
func (r *authHTTPRepo) RevokeSession(context.Context, string, time.Time) error { return nil }

type telemetryHTTPRepo struct{ rows []model.Telemetry }

func (r *telemetryHTTPRepo) InsertTelemetry(context.Context, model.Telemetry) error { return nil }
func (r *telemetryHTTPRepo) TelemetryWindow(context.Context, int64, time.Time, time.Time) ([]model.Telemetry, error) {
	return r.rows, nil
}

type scopeHTTPRepo struct{ household int64 }

func (r scopeHTTPRepo) DeviceHousehold(context.Context, int64) (int64, error) {
	return r.household, nil
}
func (r scopeHTTPRepo) PlanHousehold(context.Context, int64) (int64, error) { return r.household, nil }
func (r scopeHTTPRepo) AutomationHousehold(context.Context, int64) (int64, error) {
	return r.household, nil
}

func TestHealthAndRequestValidation(t *testing.T) {
	s := NewServer(nil, nil, nil, nil, nil, nil, nil, nil)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	s.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Header().Get("X-Request-ID") == "" {
		t.Fatalf("status=%d headers=%v body=%s", recorder.Code, recorder.Header(), recorder.Body.String())
	}
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/v1/households", nil)
	s.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid body status=%d", recorder.Code)
	}
}

func TestReadyChecksDependencyAndKeepsRequestID(t *testing.T) {
	s := NewServer(nil, nil, nil, nil, nil, nil, nil, nil)
	s.Readiness = func(context.Context) error { return errors.New("postgres unavailable") }
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	request.Header.Set("X-Request-ID", "req-ready")
	s.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable || recorder.Header().Get("X-Request-ID") != "req-ready" || strings.Contains(recorder.Body.String(), "postgres unavailable") {
		t.Fatalf("status=%d headers=%v body=%s", recorder.Code, recorder.Header(), recorder.Body.String())
	}

	s.Readiness = func(context.Context) error { return nil }
	recorder = httptest.NewRecorder()
	s.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"request_id":"req-ready"`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestReadyCancelsProbeWhenClientDisconnects(t *testing.T) {
	s := NewServer(nil, nil, nil, nil, nil, nil, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	probeStarted := make(chan struct{})
	probeCtxC := make(chan context.Context, 1)
	blocked := make(chan struct{})
	s.Readiness = func(probeCtx context.Context) error {
		probeCtxC <- probeCtx
		close(probeStarted)
		select {
		case <-probeCtx.Done():
			return probeCtx.Err()
		case <-blocked:
			return nil
		}
	}
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil).WithContext(ctx)
	request.Header.Set("X-Request-ID", "req-disconnect")
	recorder := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		s.Handler().ServeHTTP(recorder, request)
		close(done)
	}()
	<-probeStarted
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("ready handler did not return after client disconnect")
	}
	select {
	case probeCtx := <-probeCtxC:
		select {
		case <-probeCtx.Done():
		case <-time.After(time.Second):
			t.Fatal("probe context was not cancelled after client disconnect")
		}
	case <-time.After(time.Second):
		t.Fatal("probe was never invoked")
	}
	close(blocked)
}

func TestAuthenticatedTelemetryIsTenantScoped(t *testing.T) {
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	authRepo := &authHTTPRepo{
		member:  model.Member{ID: 7, HouseholdID: 11, Role: model.RoleViewer, Active: true},
		session: model.Session{ID: "session-7", MemberID: 7, ExpiresAt: now.Add(time.Hour)},
	}
	auth := service.NewAuth(authRepo, service.FixedClock{Value: now})
	telemetry := service.NewTelemetry(&telemetryHTTPRepo{rows: []model.Telemetry{{ID: 1, DeviceID: 4, Sequence: 1, MeasuredAt: now}}}, service.FixedClock{Value: now})
	s := NewServer(nil, auth, nil, telemetry, nil, nil, nil, nil)
	s.Scope = scopeHTTPRepo{household: 11}
	url := "/v1/devices/4/telemetry?start=2026-08-23T11:00:00Z&end=2026-08-23T13:00:00Z"
	request := httptest.NewRequest(http.MethodGet, url, nil)
	request.Header.Set("Authorization", "Bearer session-7")
	recorder := httptest.NewRecorder()
	s.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"Sequence":1`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	s.Scope = scopeHTTPRepo{household: 22}
	recorder = httptest.NewRecorder()
	s.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden || strings.Contains(recorder.Body.String(), `"Sequence":1`) {
		t.Fatalf("cross-household status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestRecoveryDoesNotAppendErrorAfterCommittedResponse(t *testing.T) {
	handler := recoverer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("accepted"))
		panic("after commit")
	}), nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusAccepted || recorder.Body.String() != "accepted" {
		t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}

func TestBusinessValidationErrorsMapToStableHTTPStatuses(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
	}{
		{name: "invalid telemetry", err: errors.Join(model.ErrInvalid, errors.New("power must be non-negative")), status: http.StatusBadRequest},
		{name: "state conflict", err: errors.Join(model.ErrConflict, errors.New("device is not pending")), status: http.StatusConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ErrorStatus(tc.err); got != tc.status {
				t.Fatalf("status=%d want=%d", got, tc.status)
			}
		})
	}
}
