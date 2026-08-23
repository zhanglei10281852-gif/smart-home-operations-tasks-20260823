package transport

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

type blockingBody struct {
	released chan struct{}
	active   *atomic.Int64
}

func (b *blockingBody) Read([]byte) (int, error) { return 0, io.EOF }
func (b *blockingBody) Close() error {
	<-b.released
	b.active.Add(-1)
	return nil
}

type oneSlotRoundTripper struct {
	active atomic.Int64
	body   *blockingBody
}

func (r *oneSlotRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	if r.active.Load() != 0 {
		return nil, errors.New("transport slot still owned by previous response")
	}
	r.active.Add(1)
	r.body.active = &r.active
	return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: r.body, Header: make(http.Header)}, nil
}

func TestRetryWaitsForPreviousResponseBodyRelease(t *testing.T) {
	body := &blockingBody{released: make(chan struct{})}
	roundTripper := &oneSlotRoundTripper{body: body}
	client := &Client{HTTP: &http.Client{Transport: roundTripper}, BaseURL: "https://gateway.local", Retry: 2, Backoff: time.Millisecond}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- client.Do(ctx, Request{Method: http.MethodGet, Path: "/health"}, nil) }()
	select {
	case <-result:
		close(body.released)
		t.Fatal("retry started before previous response body was released")
	case <-time.After(20 * time.Millisecond):
		close(body.released)
	}
	if err := <-result; err == nil {
		t.Fatal("expected remote failure")
	}
}
