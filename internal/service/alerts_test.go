package service

import (
	"context"
	"testing"
	"time"

	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
)

type alertRepo struct {
	alert model.Alert
	from  string
	to    string
	at    time.Time
}

func (r *alertRepo) CreateAlert(context.Context, model.Alert) (model.Alert, error) {
	return r.alert, nil
}
func (r *alertRepo) GetAlert(context.Context, int64) (model.Alert, error) { return r.alert, nil }
func (r *alertRepo) TransitionAlert(_ context.Context, _ int64, from, to string, at time.Time) error {
	r.from, r.to, r.at = from, to, at
	return nil
}

func TestResolveAlertUsesPersistedStateMachine(t *testing.T) {
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	repository := &alertRepo{alert: model.Alert{ID: 3, State: "acknowledged"}}
	service := NewAlerts(repository, FixedClock{Value: now})
	if err := service.Resolve(context.Background(), 3); err != nil {
		t.Fatal(err)
	}
	if repository.from != "acknowledged" || repository.to != "resolved" || !repository.at.Equal(now) {
		t.Fatalf("transition=%s->%s at=%v", repository.from, repository.to, repository.at)
	}
	repository.alert.State = "resolved"
	if err := service.Resolve(context.Background(), 3); err == nil {
		t.Fatal("resolved alert was transitioned twice")
	}
}
