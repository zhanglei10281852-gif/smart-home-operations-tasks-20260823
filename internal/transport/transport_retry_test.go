package transport

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestClientRetriesWithCompleteBodyAndClosesResponses(t *testing.T) {
	var mu sync.Mutex
	var bodies []string
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		mu.Lock()
		bodies = append(bodies, string(data))
		attempts++
		attempt := attempts
		mu.Unlock()
		if attempt < 3 {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = io.WriteString(w, strings.Repeat("x", 32))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"accepted":true}`)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	client.Backoff = 0
	var response struct{ Accepted bool }
	if err := client.Do(context.Background(), Request{Method: http.MethodPost, Body: map[string]string{"command": "on"}}, &response); err != nil {
		t.Fatal(err)
	}
	if !response.Accepted || len(bodies) != 3 {
		t.Fatalf("response=%+v bodies=%v", response, bodies)
	}
	for _, body := range bodies {
		if body != `{"command":"on"}` {
			t.Fatalf("retry body=%q", body)
		}
	}
}

func TestWebhookSendsStableIdempotencyKey(t *testing.T) {
	var got string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Idempotency-Key")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	client.HTTP = server.Client()
	webhook := Webhook{Client: client, URL: server.URL}
	if err := webhook.SendWithKey(context.Background(), map[string]int{"id": 7}, "outbox-7"); err != nil {
		t.Fatal(err)
	}
	if got != "outbox-7" {
		t.Fatalf("idempotency key=%q", got)
	}
}

func TestClientCancelDuringBackoffStopsRetries(t *testing.T) {
	var mu sync.Mutex
	var attempts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts++
		mu.Unlock()
		// First attempt: server-side failure that would normally trigger a retry.
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	client.Backoff = time.Second

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel during the backoff window between the first and second attempt.
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := client.Do(ctx, Request{Method: http.MethodPost, Body: map[string]string{"command": "on"}}, nil)
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v want context.Canceled", err)
	}
	if elapsed >= client.Backoff {
		t.Fatalf("elapsed=%v backoff=%v; cancellation did not stop the timer", elapsed, client.Backoff)
	}
	if got := clientAttempts(&mu, &attempts); got != 1 {
		t.Fatalf("attempts=%d want 1; cancellation reached the remote a second time", got)
	}
}

func clientAttempts(mu *sync.Mutex, attempts *int) int {
	mu.Lock()
	defer mu.Unlock()
	return *attempts
}
