package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCanceledReadinessRequestStopsOwnedProbe(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	probeCanceled := make(chan bool, 1)
	server := &Server{Readiness: func(ctx context.Context) error {
		close(started)
		select {
		case <-ctx.Done():
			probeCanceled <- true
			return ctx.Err()
		case <-release:
			probeCanceled <- ctx.Err() != nil
			return nil
		}
	}}
	ctx, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil).WithContext(ctx)
	handlerDone := make(chan struct{})
	go func() {
		server.Handler().ServeHTTP(httptest.NewRecorder(), request)
		close(handlerDone)
	}()

	<-started
	cancel()
	<-handlerDone
	close(release)
	if canceled := <-probeCanceled; !canceled {
		t.Fatal("readiness handler returned while its dependency probe kept a live context")
	}
}
