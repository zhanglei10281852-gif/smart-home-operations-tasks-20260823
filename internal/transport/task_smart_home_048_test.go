package transport

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type retryCancelRoundTripper struct {
	calls atomic.Int32
	first chan struct{}
}

func (r *retryCancelRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	call := r.calls.Add(1)
	if call == 1 {
		close(r.first)
		return &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(strings.NewReader("retry")), Header: make(http.Header)}, nil
	}
	return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
}

func TestWebhookRetryStopsWhenCallerCancels(t *testing.T) {
	roundTripper := &retryCancelRoundTripper{first: make(chan struct{})}
	client := &Client{HTTP: &http.Client{Transport: roundTripper}, BaseURL: "https://home.invalid", Retry: 2, Backoff: 40 * time.Millisecond}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- client.Do(ctx, Request{Method: http.MethodPost, Path: "/events", Body: map[string]string{"kind": "alarm"}}, nil)
	}()

	<-roundTripper.first
	cancel()
	err := <-done
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled webhook retry returned %v", err)
	}
	if got := roundTripper.calls.Load(); got != 1 {
		t.Fatalf("canceled webhook issued %d HTTP requests, want 1", got)
	}
}
