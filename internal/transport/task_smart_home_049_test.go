package transport

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type closeTrackingBody struct {
	io.Reader
	closed bool
}

func (b *closeTrackingBody) Close() error {
	b.closed = true
	return nil
}

type malformedSuccessTransport struct{ body *closeTrackingBody }

func (t malformedSuccessTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: t.body, Header: make(http.Header)}, nil
}

func TestMalformedSuccessResponseStillReleasesConnection(t *testing.T) {
	body := &closeTrackingBody{Reader: strings.NewReader(`{"device":`)}
	client := &Client{HTTP: &http.Client{Transport: malformedSuccessTransport{body: body}}, BaseURL: "https://gateway.invalid", Retry: 1}
	var output map[string]any
	err := client.Do(context.Background(), Request{Method: http.MethodGet, Path: "/device"}, &output)
	if err == nil {
		t.Fatal("malformed gateway response unexpectedly decoded")
	}
	if errors.Is(err, context.Canceled) {
		t.Fatalf("malformed response returned unrelated cancellation: %v", err)
	}
	if !body.closed {
		t.Fatal("malformed 2xx response body was not closed")
	}
}
