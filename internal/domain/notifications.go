package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

type CollectingNotifier struct{ Messages []Notification }

func (n *CollectingNotifier) Send(_ context.Context, msg Notification) error {
	n.Messages = append(n.Messages, msg)
	return nil
}
