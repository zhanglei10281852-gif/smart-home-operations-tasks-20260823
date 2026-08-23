package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Notification struct {
	Recipient, Channel, Subject, Body string
	Metadata                          map[string]string
}
type Notifier interface {
	Send(context.Context, Notification) error
}

func ValidateNotification(n Notification) error {
	if strings.TrimSpace(n.Recipient) == "" || strings.TrimSpace(n.Subject) == "" || strings.TrimSpace(n.Body) == "" {
		return fmt.Errorf("notification fields required")
	}
	switch n.Channel {
	case "email", "push", "webhook":
		return nil
	default:
		return fmt.Errorf("unsupported notification channel")
	}
}
func Send(ctx context.Context, n Notifier, msg Notification) error {
	if err := ValidateNotification(msg); err != nil {
		return err
	}
	if n == nil {
		return errors.New("notifier is not configured")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return n.Send(ctx, msg)
}

func SendHedged(ctx context.Context, n Notifier, msg Notification, hedgeAfter time.Duration) error {
	if hedgeAfter <= 0 {
		return Send(ctx, n, msg)
	}
	result := make(chan error, 2)
	go func() { result <- Send(ctx, n, msg) }()
	timer := time.NewTimer(hedgeAfter)
	defer timer.Stop()
	select {
	case err := <-result:
		return err
	case <-timer.C:
		go func() { result <- Send(ctx, n, msg) }()
		return <-result
	case <-ctx.Done():
		return ctx.Err()
	}
}

type CollectingNotifier struct{ Messages []Notification }

func (n *CollectingNotifier) Send(_ context.Context, msg Notification) error {
	n.Messages = append(n.Messages, msg)
	return nil
}
