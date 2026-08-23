package observability

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type plainReader struct{ r io.Reader }

func (p *plainReader) Read(b []byte) (int, error) { return p.r.Read(b) }

func TestAccessLogDoesNotAppendErrorAfterStreamedResponse(t *testing.T) {
	handler := AccessLog(nil, &Metrics{}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.Copy(w, &plainReader{r: strings.NewReader(`{"ok":true}`)})
		panic("after stream")
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != `{"ok":true}` {
		t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}
