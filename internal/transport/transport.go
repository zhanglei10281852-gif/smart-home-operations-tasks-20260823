package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	if c.HTTP == nil {
		c.HTTP = &http.Client{Timeout: 10 * time.Second}
	}
	if req.Method == "" {
		req.Method = http.MethodGet
	}
	target, err := url.JoinPath(c.BaseURL, req.Path)
	if err != nil {
		return err
	}
	var body strings.Reader
	if req.Body != nil {
		data, e := json.Marshal(req.Body)
		if e != nil {
			return e
		}
		body = *strings.NewReader(string(data))
	}
	attempts := c.Retry
	if attempts < 1 {
		attempts = 1
	}
	var last error
	for attempt := 1; attempt <= attempts; attempt++ {
		httpReq, err := http.NewRequestWithContext(ctx, req.Method, target, &body)
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
		resp, err := c.HTTP.Do(httpReq)
		if err != nil {
			last = err
		} else {
			defer resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				if out == nil {
					return nil
				}
				return json.NewDecoder(resp.Body).Decode(out)
			}
			last = fmt.Errorf("remote status %d", resp.StatusCode)
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
	if err := CheckURL(w.URL); err != nil {
		return err
	}
	return w.Client.Do(ctx, Request{Method: http.MethodPost, Path: w.URL, Body: event}, nil)
}
