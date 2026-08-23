package observability

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRegistryCountersAndExport(t *testing.T) {
	r := NewRegistry()
	c := r.Counter("requests_total", map[string]string{"route": "/healthz"})
	c.Add(2)
	c.Add(3)
	if c.Value() != 5 || r.Counter("requests_total", nil) != c {
		t.Fatal("counter was not stable")
	}
	snapshot := r.Snapshot(time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC))
	if len(snapshot) != 1 || snapshot[0].Name != "requests_total" || snapshot[0].Value != 5 {
		t.Fatalf("snapshot=%+v", snapshot)
	}
	exported, err := r.Export(context.Background(), time.Now())
	if err != nil || exported != "requests_total 5\n" {
		t.Fatalf("export=%q err=%v", exported, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.Export(ctx, time.Now()); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel err=%v", err)
	}
}

func TestObserveClampsFutureStartTime(t *testing.T) {
	metrics := &Metrics{}
	metrics.Observe(time.Now().Add(time.Hour), false)
	requests, failures, latency := metrics.Snapshot()
	if requests != 1 || failures != 0 || latency != 0 {
		t.Fatalf("requests=%d failures=%d latency=%v", requests, failures, latency)
	}
}

func TestAccessLogCountsServerErrorResponses(t *testing.T) {
	metrics := &Metrics{}
	handler := AccessLog(nil, metrics, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/readyz", nil))
	requests, failures, _ := metrics.Snapshot()
	if requests != 1 || failures != 1 {
		t.Fatalf("requests=%d failures=%d", requests, failures)
	}
}
