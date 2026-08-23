package transport

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
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
