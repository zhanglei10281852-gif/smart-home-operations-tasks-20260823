package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/service"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Server struct {
	Households *service.HouseholdService
	Auth       *service.AuthService
	Devices    *service.DeviceService
	Telemetry  *service.TelemetryService
	Energy     *service.EnergyService
	Automation *service.AutomationService
	Reports    *service.ReportService
	Logger     *slog.Logger
}

func NewServer(h *service.HouseholdService, a *service.AuthService, d *service.DeviceService, t *service.TelemetryService, e *service.EnergyService, au *service.AutomationService, r *service.ReportService, l *slog.Logger) *Server {
	return &Server{Households: h, Auth: a, Devices: d, Telemetry: t, Energy: e, Automation: au, Reports: r, Logger: l}
}
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
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
	mux.HandleFunc("POST /v1/automations/{id}/runs", s.run)
	return accessLog(withRequestContext(contentTypeJSON(limitBody(requestID(recoverer(mux, s.Logger)), 1<<20))), s.Logger)
}
func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if id == "" {
			id = "req-" + strconv.FormatInt(time.Now().UnixNano(), 10)
		}
		if err := validateRequestID(id); err != nil {
			writeError(w, model.ErrInvalid)
			return
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r)
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
		defer func() {
			if v := recover(); v != nil {
				if l != nil {
					l.Error("panic recovered", "value", v)
				}
				writeError(w, errors.New("internal server error"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok"})
}
func decode(r *http.Request, v any) error {
	return json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(v)
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, err error) {
	status := 400
	if errors.Is(err, model.ErrForbidden) {
		status = 403
	}
	if errors.Is(err, model.ErrNotFound) {
		status = 404
	}
	if errors.Is(err, model.ErrConflict) {
		status = 409
	}
	if !errors.Is(err, model.ErrInvalid) && status == 400 {
		status = 500
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
func idParam(r *http.Request) (int64, error) { return strconv.ParseInt(r.PathValue("id"), 10, 64) }
func (s *Server) createHousehold(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name, Timezone string
		BudgetCents    int64
	}
	if err := decode(r, &in); err != nil {
		writeError(w, model.ErrInvalid)
		return
	}
	h, err := s.Households.Create(r.Context(), in.Name, in.Timezone, in.BudgetCents)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 201, h)
}
func (s *Server) addMember(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r)
	if err != nil {
		writeError(w, model.ErrInvalid)
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
	if err := s.Auth.Logout(r.Context(), r.PathValue("id")); err != nil {
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
