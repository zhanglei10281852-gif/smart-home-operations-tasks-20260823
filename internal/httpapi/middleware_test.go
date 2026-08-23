package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
)

func TestRequestIDContextAndAccessLog(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	request.Header.Set("X-Request-ID", "req-test")
	recorder := httptest.NewRecorder()
	called := false
	handler := requestID(accessLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if RequestID(r.Context()) != "req-test" {
			t.Fatalf("request id=%q", RequestID(r.Context()))
		}
		w.WriteHeader(http.StatusNoContent)
	}), slog.New(slog.NewTextHandler(io.Discard, nil))))
	handler.ServeHTTP(recorder, request)
	if !called || recorder.Code != http.StatusNoContent || recorder.Header().Get("X-Request-ID") != "req-test" {
		t.Fatalf("called=%v status=%d headers=%v", called, recorder.Code, recorder.Header())
	}
}

func TestDecodeStrictRejectsUnknownAndTrailing(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"ok"} {"extra":true}`))
	var value struct {
		Name string `json:"name"`
	}
	if err := decodeStrict(request, &value); err == nil {
		t.Fatal("trailing JSON accepted")
	}
	request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"unexpected":true}`))
	if err := decodeStrict(request, &value); err == nil {
		t.Fatal("unknown JSON field accepted")
	}
	request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"ok"} trailing`))
	if err := decodeStrict(request, &value); err == nil {
		t.Fatal("malformed trailing content accepted")
	}
}

func TestStructuredErrorMapping(t *testing.T) {
	if ErrorCode(model.ErrConflict) != "conflict" || ErrorStatus(model.ErrNotFound) != http.StatusNotFound || ErrorStatus(errors.New("x")) != http.StatusInternalServerError {
		t.Fatal("error mapping incorrect")
	}
	recorder := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = r.WithContext(context.WithValue(r.Context(), requestContextKey, "req-7"))
	WriteStructuredError(recorder, r, model.ErrInvalid)
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "req-7") {
		t.Fatalf("response=%s status=%d", recorder.Body.String(), recorder.Code)
	}
}
