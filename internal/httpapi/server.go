package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/service"
)

type ResourceScope interface {
	DeviceHousehold(context.Context, int64) (int64, error)
	PlanHousehold(context.Context, int64) (int64, error)
	AutomationHousehold(context.Context, int64) (int64, error)
}

type Server struct {
	Households *service.HouseholdService
	Auth       *service.AuthService
	Devices    *service.DeviceService
	Telemetry  *service.TelemetryService
	Energy     *service.EnergyService
	Automation *service.AutomationService
	Reports    *service.ReportService
	Scope      ResourceScope
	Readiness  func(context.Context) error
	Logger     *slog.Logger
}

func NewServer(h *service.HouseholdService, a *service.AuthService, d *service.DeviceService, t *service.TelemetryService, e *service.EnergyService, au *service.AutomationService, r *service.ReportService, l *slog.Logger) *Server {
	return &Server{Households: h, Auth: a, Devices: d, Telemetry: t, Energy: e, Automation: au, Reports: r, Logger: l}
}
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /readyz", s.ready)
	mux.HandleFunc("POST /v1/households", s.createHousehold)
	mux.HandleFunc("POST /v1/households/{id}/members", s.addMember)
	mux.HandleFunc("POST /v1/households/{id}/sessions", s.login)
	mux.HandleFunc("DELETE /v1/sessions/{id}", s.logout)
	mux.HandleFunc("POST /v1/households/{id}/devices", s.enroll)
	mux.HandleFunc("POST /v1/devices/{id}/pair", s.pair)
	mux.HandleFunc("POST /v1/devices/{id}/enable", s.enable)
	mux.HandleFunc("POST /v1/devices/{id}/disable", s.disable)
	mux.HandleFunc("POST /v1/devices/{id}/telemetry", s.telemetry)
	mux.HandleFunc("GET /v1/devices/{id}/telemetry", s.window)
	mux.HandleFunc("POST /v1/households/{id}/plans", s.plan)
	mux.HandleFunc("POST /v1/plans/{id}/schedule", s.schedule)
	mux.HandleFunc("POST /v1/plans/{id}/start", s.start)
	mux.HandleFunc("POST /v1/plans/{id}/complete", s.complete)
	mux.HandleFunc("POST /v1/households/{id}/automations", s.automation)
	mux.HandleFunc("POST /v1/automations/{id}/activate", s.activateAutomation)
	mux.HandleFunc("POST /v1/automations/{id}/pause", s.pauseAutomation)
	mux.HandleFunc("POST /v1/automations/{id}/runs", s.run)
	return requestID(accessLog(recoverer(contentTypeJSON(limitBody(mux, 1<<20)), s.Logger), s.Logger))
}
func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if id == "" {
			id = "req-" + uuid.NewString()
		}
		if err := validateRequestID(id); err != nil {
			id = "req-" + uuid.NewString()
			w.Header().Set("X-Request-ID", id)
			writeError(w, model.ErrInvalid)
			return
		}
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), requestContextKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
func validateRequestID(v string) error {
	if v == "" || len(v) > 128 {
		return model.ErrInvalid
	}
	for _, r := range v {
		if r < ' ' || r == '\u007f' {
			return model.ErrInvalid
		}
	}
	return nil
}
func recoverer(next http.Handler, l *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tracked := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			if v := recover(); v != nil {
				if l != nil {
					l.Error("panic recovered", "value", v)
				}
				if !tracked.committed {
					writeError(tracked, errors.New("internal server error"))
				}
			}
		}()
		next.ServeHTTP(tracked, r)
	})
}
func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok"})
}
func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if s.Readiness == nil {
		writeError(w, errors.New("readiness dependency is not configured"))
		return
	}
	probeDone, cancelProbe := startReadinessProbe(r.Context(), 2*time.Second, s.Readiness)
	defer cancelProbe()
	var err error
	select {
	case err = <-probeDone:
	case <-r.Context().Done():
		return
	}
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable", "request_id": RequestID(r.Context())})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready", "request_id": RequestID(r.Context())})
}
func decode(r *http.Request, v any) error {
	return decodeStrict(r, v)
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, err error) {
	status := ErrorStatus(err)
	message := "internal server error"
	if status < 500 {
		message = err.Error()
	}
	writeJSON(w, status, ErrorBody{Code: ErrorCode(err), Message: message, RequestID: w.Header().Get("X-Request-ID")})
}
func idParam(r *http.Request) (int64, error) { return strconv.ParseInt(r.PathValue("id"), 10, 64) }

func bearerSession(r *http.Request) (string, error) {
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return "", model.ErrForbidden
	}
	return parts[1], nil
}

func (s *Server) authorize(r *http.Request, required model.Role) (model.Member, error) {
	if s.Auth == nil {
		return model.Member{}, errors.New("authentication service is not configured")
	}
	sessionID, err := bearerSession(r)
	if err != nil {
		return model.Member{}, err
	}
	return s.Auth.Authorize(r.Context(), sessionID, required)
}

func (s *Server) authorizeHousehold(r *http.Request, householdID int64, required model.Role) (model.Member, error) {
	member, err := s.authorize(r, required)
	if err != nil {
		return model.Member{}, err
	}
	if householdID <= 0 || member.HouseholdID != householdID {
		return model.Member{}, model.ErrForbidden
	}
	return member, nil
}

func (s *Server) authorizeResource(r *http.Request, id int64, required model.Role, lookup func(context.Context, int64) (int64, error)) (model.Member, error) {
	if s.Scope == nil || lookup == nil {
		return model.Member{}, errors.New("resource authorization is not configured")
	}
	member, err := s.authorize(r, required)
	if err != nil {
		return model.Member{}, err
	}
	householdID, err := lookup(r.Context(), id)
	if err != nil {
		return model.Member{}, err
	}
	if member.HouseholdID != householdID {
		return model.Member{}, model.ErrForbidden
	}
	return member, nil
}

func (s *Server) requireResource(w http.ResponseWriter, r *http.Request, id int64, required model.Role, lookup func(context.Context, int64) (int64, error)) bool {
	if _, err := s.authorizeResource(r, id, required, lookup); err != nil {
		writeError(w, fmt.Errorf("authorize resource: %w", err))
		return false
	}
	return true
}

func (s *Server) deviceHousehold(ctx context.Context, id int64) (int64, error) {
	if s.Scope == nil {
		return 0, errors.New("resource authorization is not configured")
	}
	return s.Scope.DeviceHousehold(ctx, id)
}

func (s *Server) planHousehold(ctx context.Context, id int64) (int64, error) {
	if s.Scope == nil {
		return 0, errors.New("resource authorization is not configured")
	}
	return s.Scope.PlanHousehold(ctx, id)
}

func (s *Server) automationHousehold(ctx context.Context, id int64) (int64, error) {
	if s.Scope == nil {
		return 0, errors.New("resource authorization is not configured")
	}
	return s.Scope.AutomationHousehold(ctx, id)
}
func (s *Server) createHousehold(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name, Timezone            string
		BudgetCents               int64
		OwnerEmail, OwnerPassword string
	}
	if err := decode(r, &in); err != nil {
		writeError(w, model.ErrInvalid)
		return
	}
	if s.Households == nil {
		writeError(w, errors.New("household service is not configured"))
		return
	}
	h, owner, err := s.Households.Onboard(r.Context(), in.Name, in.Timezone, in.BudgetCents, in.OwnerEmail, in.OwnerPassword)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 201, map[string]any{"household": h, "owner": owner})
}
func (s *Server) addMember(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r)
	if err != nil {
		writeError(w, model.ErrInvalid)
		return
	}
	if _, err = s.authorizeHousehold(r, id, model.RoleOwner); err != nil {
		writeError(w, err)
		return
	}
	var in struct {
		Email, Password string
		Role            model.Role
	}
	if err = decode(r, &in); err != nil {
		writeError(w, model.ErrInvalid)
		return
	}
	m, err := s.Households.AddMember(r.Context(), id, in.Email, in.Password, in.Role)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 201, m)
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r)
	if err != nil {
		writeError(w, model.ErrInvalid)
		return
	}
	var in struct{ Email, Password string }
	if err = decode(r, &in); err != nil {
		writeError(w, model.ErrInvalid)
		return
	}
	sess, member, err := s.Auth.Login(r.Context(), id, in.Email, in.Password)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"session": sess, "member": member})
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	sessionID, err := bearerSession(r)
	if err != nil || sessionID != r.PathValue("id") {
		writeError(w, model.ErrForbidden)
		return
	}
	if err := s.Auth.Logout(r.Context(), sessionID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) enroll(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r)
	if err != nil {
		writeError(w, model.ErrInvalid)
		return
	}
	if _, err = s.authorizeHousehold(r, id, model.RoleOperator); err != nil {
		writeError(w, err)
		return
	}
	var in struct {
		ExternalID   string
		Kind         model.DeviceKind
		Firmware     string
		Capabilities []string
	}
	if err = decode(r, &in); err != nil {
		writeError(w, model.ErrInvalid)
		return
	}
	d, err := s.Devices.Enroll(r.Context(), id, in.ExternalID, in.Kind, in.Firmware, in.Capabilities)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 201, d)
}
func (s *Server) pair(w http.ResponseWriter, r *http.Request) {
	s.deviceTransition(w, r, s.Devices.Pair)
}
func (s *Server) enable(w http.ResponseWriter, r *http.Request) {
	s.deviceTransition(w, r, s.Devices.Enable)
}
func (s *Server) disable(w http.ResponseWriter, r *http.Request) {
	s.deviceTransition(w, r, s.Devices.Disable)
}
func (s *Server) deviceTransition(w http.ResponseWriter, r *http.Request, fn func(context.Context, int64) error) {
	id, err := idParam(r)
	if err != nil {
		writeError(w, model.ErrInvalid)
		return
	}
	if !s.requireResource(w, r, id, model.RoleOperator, s.deviceHousehold) {
		return
	}
	if err = fn(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) telemetry(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r)
	if err != nil {
		writeError(w, model.ErrInvalid)
		return
	}
	if !s.requireResource(w, r, id, model.RoleOperator, s.deviceHousehold) {
		return
	}
	var in struct {
		Sequence     int64
		PowerWatts   float64
		TemperatureC *float64
		MeasuredAt   time.Time
	}
	if err = decode(r, &in); err != nil {
		writeError(w, model.ErrInvalid)
		return
	}
	err = s.Telemetry.Record(r.Context(), model.Telemetry{DeviceID: id, Sequence: in.Sequence, PowerWatts: in.PowerWatts, TemperatureC: in.TemperatureC, MeasuredAt: in.MeasuredAt})
	if err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) window(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r)
	if err != nil {
		writeError(w, model.ErrInvalid)
		return
	}
	if !s.requireResource(w, r, id, model.RoleViewer, s.deviceHousehold) {
		return
	}
	start, err1 := time.Parse(time.RFC3339, r.URL.Query().Get("start"))
	end, err2 := time.Parse(time.RFC3339, r.URL.Query().Get("end"))
	if err1 != nil || err2 != nil {
		writeError(w, model.ErrInvalid)
		return
	}
	rows, err := s.Telemetry.Window(r.Context(), id, start, end)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, rows)
}
func (s *Server) plan(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r)
	if err != nil {
		writeError(w, model.ErrInvalid)
		return
	}
	if _, err = s.authorizeHousehold(r, id, model.RoleOperator); err != nil {
		writeError(w, err)
		return
	}
	var in struct {
		Name             string
		BudgetCents      int64
		StartsAt, EndsAt time.Time
		Devices          []model.PlanDevice
	}
	if err = decode(r, &in); err != nil {
		writeError(w, model.ErrInvalid)
		return
	}
	p, err := s.Energy.Draft(r.Context(), model.EnergyPlan{HouseholdID: id, Name: in.Name, BudgetCents: in.BudgetCents, StartsAt: in.StartsAt, EndsAt: in.EndsAt}, in.Devices)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 201, p)
}
func (s *Server) planState(w http.ResponseWriter, r *http.Request, fn func(context.Context, int64) error) {
	id, err := idParam(r)
	if err != nil {
		writeError(w, model.ErrInvalid)
		return
	}
	if !s.requireResource(w, r, id, model.RoleOperator, s.planHousehold) {
		return
	}
	if err = fn(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) schedule(w http.ResponseWriter, r *http.Request) {
	s.planState(w, r, s.Energy.Schedule)
}
func (s *Server) start(w http.ResponseWriter, r *http.Request) { s.planState(w, r, s.Energy.Start) }
func (s *Server) complete(w http.ResponseWriter, r *http.Request) {
	s.planState(w, r, s.Energy.Complete)
}
func (s *Server) automation(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r)
	if err != nil {
		writeError(w, model.ErrInvalid)
		return
	}
	if _, err = s.authorizeHousehold(r, id, model.RoleOperator); err != nil {
		writeError(w, err)
		return
	}
	var in struct {
		Name, TriggerKind string
		Actions           []model.AutomationAction
	}
	if err = decode(r, &in); err != nil {
		writeError(w, model.ErrInvalid)
		return
	}
	a, err := s.Automation.Create(r.Context(), model.Automation{HouseholdID: id, Name: in.Name, TriggerKind: in.TriggerKind}, in.Actions)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 201, a)
}
func (s *Server) run(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r)
	if err != nil {
		writeError(w, model.ErrInvalid)
		return
	}
	if !s.requireResource(w, r, id, model.RoleOperator, s.automationHousehold) {
		return
	}
	var in struct{ IdempotencyKey string }
	if err = decode(r, &in); err != nil {
		writeError(w, model.ErrInvalid)
		return
	}
	run, err := s.Automation.Queue(r.Context(), id, in.IdempotencyKey)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 202, run)
}

func (s *Server) activateAutomation(w http.ResponseWriter, r *http.Request) {
	s.automationState(w, r, s.Automation.Activate)
}

func (s *Server) pauseAutomation(w http.ResponseWriter, r *http.Request) {
	s.automationState(w, r, s.Automation.Pause)
}

func (s *Server) automationState(w http.ResponseWriter, r *http.Request, transition func(context.Context, int64) error) {
	id, err := idParam(r)
	if err != nil {
		writeError(w, model.ErrInvalid)
		return
	}
	if !s.requireResource(w, r, id, model.RoleOperator, s.automationHousehold) {
		return
	}
	if transition == nil {
		writeError(w, errors.New("automation transition is not configured"))
		return
	}
	if err := transition(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
