package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
)

type contextKey string

const requestContextKey contextKey = "request-id"

func RequestID(ctx context.Context) string {
	value, _ := ctx.Value(requestContextKey).(string)
	return value
}

func accessLog(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		wrapped := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(wrapped, r)
		if logger != nil {
			logger.Info("http request", "method", r.Method, "path", r.URL.Path, "status", wrapped.status, "duration_ms", time.Since(started).Milliseconds(), "request_id", RequestID(r.Context()))
		}
	})
}

type statusWriter struct {
	http.ResponseWriter
	status    int
	committed bool
}

func (w *statusWriter) WriteHeader(status int) {
	if w.committed {
		return
	}
	w.status = status
	w.committed = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(data []byte) (int, error) {
	if !w.committed {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func contentTypeJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func limitBody(next http.Handler, bytes int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, bytes)
		next.ServeHTTP(w, r)
	})
}

func decodeStrict(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return model.ErrInvalid
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return model.ErrInvalid
	}
	return nil
}
