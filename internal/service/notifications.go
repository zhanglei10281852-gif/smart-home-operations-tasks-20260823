package service

import (
	"context"
	"fmt"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/domain"
	"time"
)

type NotificationService struct{ Notifier domain.Notifier }

func NewNotifications(n domain.Notifier) *NotificationService {
	return &NotificationService{Notifier: n}
}
func (s *NotificationService) SendAlert(ctx context.Context, recipient, code, severity string) error {
	return domain.SendHedged(ctx, s.Notifier, domain.Notification{Recipient: recipient, Channel: "push", Subject: "Smart home alert: " + code, Body: fmt.Sprintf("Severity: %s", severity)}, 10*time.Millisecond)
}
func (s *NotificationService) SendInvite(ctx context.Context, recipient, household string) error {
	return domain.Send(ctx, s.Notifier, domain.Notification{Recipient: recipient, Channel: "email", Subject: "Household invitation", Body: "You were invited to " + household})
}
