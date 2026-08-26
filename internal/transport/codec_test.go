package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestEnvelopeRoundTripAndErrors(t *testing.T) {
	raw, err := EncodeEnvelope("req-1", map[string]any{"device": 4})
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]any
	id, err := DecodeEnvelope(raw, &data)
	if err != nil || id != "req-1" || data["device"] != float64(4) {
		t.Fatalf("id=%s data=%v err=%v", id, data, err)
	}
	if _, err := EncodeEnvelope("", nil); err == nil {
		t.Fatal("empty request id accepted")
	}
	bad, _ := EncodeEnvelope("req-2", nil)
	bad = bytes.Replace(bad, []byte(`"data":null`), []byte(`"error":{"code":"bad","message":"no"}`), 1)
	if _, err := DecodeEnvelope([]byte(bad), &data); err == nil {
		t.Fatal("remote envelope error ignored")
	}
}

func TestStreamDecoderLimitAndCancellation(t *testing.T) {
	var values []json.RawMessage
	decoder := StreamDecoder{Reader: strings.NewReader(`{"a":1}{"b":2}`), Limit: 100}
	if err := decoder.DecodeAll(context.Background(), &values); err != nil || len(values) != 2 {
		t.Fatalf("values=%d err=%v", len(values), err)
	}
	tooLarge := StreamDecoder{Reader: strings.NewReader(`{"long":"0123456789"}`), Limit: 5}
	if err := tooLarge.DecodeAll(context.Background(), &values); err == nil {
		t.Fatal("oversized stream accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := (StreamDecoder{Reader: strings.NewReader(`{"a":1}`)}).DecodeAll(ctx, &values); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel err=%v", err)
	}
}

func TestRetryAfterParsing(t *testing.T) {
	fallback := 250 * time.Millisecond
	resp := &http.Response{Header: make(http.Header)}
	if got := RetryAfter(resp, fallback); got != fallback {
		t.Fatalf("fallback=%v", got)
	}
	resp.Header.Set("Retry-After", "3")
	if got := RetryAfter(resp, fallback); got != 3*time.Second {
		t.Fatalf("seconds=%v", got)
	}
	resp.Header.Set("Retry-After", "invalid")
	if got := RetryAfter(resp, fallback); got != fallback {
		t.Fatalf("invalid=%v", got)
	}
}
