package transport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRequestPolicyApply(t *testing.T) {
	policy := RequestPolicy{AllowedMethods: map[string]bool{http.MethodGet: true}, MaxBodyBytes: 1024, Timeout: time.Second, RequireToken: true}
	request := httptest.NewRequest(http.MethodGet, "https://example.test", nil)
	if _, err := policy.Apply(context.Background(), request); err == nil {
		t.Fatal("missing token accepted")
	}
	request.Header.Set("Authorization", "Bearer token")
	derived, err := policy.Apply(context.Background(), request)
	if err != nil || derived.Context() == request.Context() {
		t.Fatalf("derived=%v err=%v", derived, err)
	}
	request.Method = http.MethodPost
	if _, err := policy.Apply(context.Background(), request); err == nil {
		t.Fatal("disallowed method accepted")
	}
}

func TestRequestPolicyValidation(t *testing.T) {
	invalid := []RequestPolicy{{}, {AllowedMethods: map[string]bool{"PATCH": true}, MaxBodyBytes: 1, Timeout: time.Second}, {AllowedMethods: map[string]bool{"GET": true}, MaxBodyBytes: 0, Timeout: time.Second}, {AllowedMethods: map[string]bool{"GET": true}, MaxBodyBytes: 1, Timeout: 0}}
	for _, policy := range invalid {
		if err := policy.Validate(); err == nil {
			t.Fatalf("invalid policy accepted: %+v", policy)
		}
	}
	valid := RequestPolicy{AllowedMethods: map[string]bool{http.MethodGet: true, http.MethodPost: true}, MaxBodyBytes: 2048, Timeout: 5 * time.Second}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestRequestPolicyNilRequest(t *testing.T) {
	policy := RequestPolicy{AllowedMethods: map[string]bool{http.MethodGet: true}, MaxBodyBytes: 1, Timeout: time.Second}
	if _, err := policy.Apply(context.Background(), nil); err == nil {
		t.Fatal("nil request accepted")
	}
}
