package httpapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type streamOnlyReader struct{ reader io.Reader }

func (r streamOnlyReader) Read(data []byte) (int, error) { return r.reader.Read(data) }

func TestPanicAfterStreamCommitDoesNotAppendErrorEnvelope(t *testing.T) {
	handler := recoverer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := io.Copy(w, streamOnlyReader{reader: strings.NewReader(`{"status":"accepted"}`)}); err != nil {
			t.Fatalf("stream response: %v", err)
		}
		panic("gateway stream failed after commit")
	}), nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/stream", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("committed response status = %d, want 200", recorder.Code)
	}
	if got, want := recorder.Body.String(), `{"status":"accepted"}`; got != want {
		t.Fatalf("committed response was modified after panic: got %q want %q", got, want)
	}
}
