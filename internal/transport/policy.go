package transport

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type RequestPolicy struct {
	AllowedMethods map[string]bool
	MaxBodyBytes   int64
	Timeout        time.Duration
	RequireToken   bool
}

func (p RequestPolicy) Validate() error {
	if len(p.AllowedMethods) == 0 || p.MaxBodyBytes <= 0 || p.Timeout <= 0 {
		return errors.New("request policy is incomplete")
	}
	for method := range p.AllowedMethods {
		if method != http.MethodGet && method != http.MethodPost && method != http.MethodPut && method != http.MethodDelete {
			return fmt.Errorf("method %s is not supported", method)
		}
	}
	return nil
}

func (p RequestPolicy) Apply(ctx context.Context, req *http.Request) (*http.Request, context.CancelFunc, error) {
	if req == nil {
		return nil, nil, errors.New("request is nil")
	}
	if err := p.Validate(); err != nil {
		return nil, nil, err
	}
	if !p.AllowedMethods[req.Method] {
		return nil, nil, fmt.Errorf("method %s is not allowed", req.Method)
	}
	if p.RequireToken && strings.TrimSpace(req.Header.Get("Authorization")) == "" {
		return nil, nil, errors.New("authorization is required")
	}
	derived, cancel := context.WithTimeout(ctx, p.Timeout)
	return req.WithContext(derived), cancel, nil
}
