package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	HTTP    *http.Client
	BaseURL string
	Token   string
	Retry   int
	Backoff time.Duration
}

func NewClient(base string) *Client {
	base = strings.TrimRight(base, "/")
	return &Client{HTTP: &http.Client{Timeout: 10 * time.Second}, BaseURL: base, Retry: 3, Backoff: 100 * time.Millisecond}
}

type Request struct {
	Method, Path string
	Body         any
	Headers      map[string]string
}

func (c *Client) Do(ctx context.Context, req Request, out any) error {
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	if req.Method == "" {
		req.Method = http.MethodGet
	}
	target, err := url.JoinPath(c.BaseURL, req.Path)
	if err != nil {
		return err
	}
	var encodedBody []byte
	if req.Body != nil {
		data, e := json.Marshal(req.Body)
		if e != nil {
			return e
		}
		encodedBody = data
	}
	attempts := c.Retry
	if attempts < 1 {
		attempts = 1
	}
	var last error
	for attempt := 1; attempt <= attempts; attempt++ {
		// Honor caller cancellation before building the next attempt: a cancel
		// that landed during the previous backoff must not reach the remote.
		if err := ctx.Err(); err != nil {
			return err
		}
		httpReq, err := http.NewRequestWithContext(ctx, req.Method, target, bytes.NewReader(encodedBody))
		if err != nil {
			return err
		}
		httpReq.Header.Set("Accept", "application/json")
		if req.Body != nil {
			httpReq.Header.Set("Content-Type", "application/json")
		}
		if c.Token != "" {
			httpReq.Header.Set("Authorization", "Bearer "+c.Token)
		}
		for k, v := range req.Headers {
			httpReq.Header.Set(k, v)
		}
		resp, err := httpClient.Do(httpReq)
		if err != nil {
			last = err
		} else {
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				if out == nil {
					return drainAndClose(resp.Body)
				}
				decodeErr := json.NewDecoder(resp.Body).Decode(out)
				closeErr := resp.Body.Close()
				if decodeErr != nil {
					return decodeErr
				}
				return closeErr
			}
			last = fmt.Errorf("remote status %d", resp.StatusCode)
			if closeErr := drainAndClose(resp.Body); closeErr != nil {
				return closeErr
			}
			if resp.StatusCode < 500 {
				return last
			}
		}
		if attempt < attempts {
			timer := time.NewTimer(c.Backoff * time.Duration(attempt))
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
	return last
}

func drainAndClose(body io.ReadCloser) error {
	if body == nil {
		return nil
	}
	_, copyErr := io.Copy(io.Discard, io.LimitReader(body, 1<<20))
	closeErr := body.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
func CheckURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return errors.New("webhook URL must use https")
	}
	return nil
}

type Webhook struct {
	Client *Client
	URL    string
}

func (w Webhook) Send(ctx context.Context, event any) error {
	return w.SendWithKey(ctx, event, "")
}

func (w Webhook) SendWithKey(ctx context.Context, event any, idempotencyKey string) error {
	if err := CheckURL(w.URL); err != nil {
		return err
	}
	if w.Client == nil {
		return errors.New("webhook client is required")
	}
	client := *w.Client
	client.BaseURL = strings.TrimRight(w.URL, "/")
	headers := map[string]string{}
	if idempotencyKey != "" {
		headers["Idempotency-Key"] = idempotencyKey
	}
	return client.Do(ctx, Request{Method: http.MethodPost, Path: "", Body: event, Headers: headers}, nil)
}
