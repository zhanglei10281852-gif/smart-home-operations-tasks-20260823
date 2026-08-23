package observability

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"
	"sync/atomic"
	"time"
)

type Metrics struct {
	Requests     atomic.Int64
	Failures     atomic.Int64
	LatencyNanos atomic.Int64
}

type responseState struct {
	http.ResponseWriter
	committed bool
	status    int
}

func (w *responseState) WriteHeader(status int) {
	if w.committed {
		return
	}
	w.committed = true
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseState) Write(data []byte) (int, error) {
	if !w.committed {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

func (w *responseState) ReadFrom(reader io.Reader) (int64, error) {
	if !w.committed {
		w.status = http.StatusOK
		w.committed = true
	}
	if rf, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		return rf.ReadFrom(reader)
	}
	return io.Copy(w.ResponseWriter, reader)
}

func (w *responseState) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (m *Metrics) Observe(start time.Time, failed bool) {
	m.Requests.Add(1)
	elapsed := time.Since(start)
	if elapsed < 0 {
		elapsed = 0
	}
	m.LatencyNanos.Add(elapsed.Nanoseconds())
	if failed {
		m.Failures.Add(1)
	}
}
func (m *Metrics) Snapshot() (int64, int64, time.Duration) {
	requests := m.Requests.Load()
	return requests, m.Failures.Load(), time.Duration(m.LatencyNanos.Load())
}
func AccessLog(logger *slog.Logger, metrics *Metrics, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		failed := false
		tracked := &responseState{ResponseWriter: w}
		defer func() {
			if recover() != nil {
				failed = true
				if logger != nil {
					logger.Error("panic", "stack", string(debug.Stack()))
				}
				if !tracked.committed {
					http.Error(tracked, "internal", http.StatusInternalServerError)
				}
			}
			if tracked.status >= http.StatusInternalServerError {
				failed = true
			}
			if metrics != nil {
				metrics.Observe(start, failed)
			}
			if logger != nil {
				logger.Info("http request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start))
			}
		}()
		next.ServeHTTP(tracked, r)
	})
}

type ContextKey string

const RequestKey ContextKey = "request_id"

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, RequestKey, id)
}
func RequestID(ctx context.Context) string { value, _ := ctx.Value(RequestKey).(string); return value }
