package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Envelope struct {
	RequestID string          `json:"request_id"`
	Data      json.RawMessage `json:"data"`
	Error     *RemoteError    `json:"error,omitempty"`
}

type RemoteError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func EncodeEnvelope(requestID string, data any) ([]byte, error) {
	if strings.TrimSpace(requestID) == "" {
		return nil, errors.New("request id is required")
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return json.Marshal(Envelope{RequestID: requestID, Data: payload})
}

func DecodeEnvelope(raw []byte, target any) (string, error) {
	var envelope Envelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return "", err
	}
	if envelope.RequestID == "" || len(envelope.Data) == 0 {
		return "", errors.New("invalid envelope")
	}
	if envelope.Error != nil {
		return envelope.RequestID, fmt.Errorf("remote %s: %s", envelope.Error.Code, envelope.Error.Message)
	}
	if err := json.Unmarshal(envelope.Data, target); err != nil {
		return "", err
	}
	return envelope.RequestID, nil
}

type StreamDecoder struct {
	Reader io.Reader
	Limit  int64
}

func (d StreamDecoder) DecodeAll(ctx context.Context, target *[]json.RawMessage) error {
	if d.Reader == nil || target == nil {
		return errors.New("decoder is not configured")
	}
	limit := d.Limit
	if limit <= 0 {
		limit = 4 << 20
	}
	data, err := io.ReadAll(io.LimitReader(d.Reader, limit+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > limit {
		return errors.New("payload exceeds limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	for {
		var value json.RawMessage
		err := decoder.Decode(&value)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		*target = append(*target, value)
	}
}

func RetryAfter(resp *http.Response, fallback time.Duration) time.Duration {
	if resp == nil {
		return fallback
	}
	value := strings.TrimSpace(resp.Header.Get("Retry-After"))
	if value == "" {
		return fallback
	}
	var seconds int
	if _, err := fmt.Sscanf(value, "%d", &seconds); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil {
		if delay := time.Until(when); delay > 0 {
			return delay
		}
	}
	return fallback
}
