package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
)

type cancellationFailureStore struct {
	finished bool
	state    model.RunState
}

func (*cancellationFailureStore) RequeueRun(context.Context, int64) error {
	return errors.New("database unavailable while requeueing")
}
func (s *cancellationFailureStore) FinishRun(_ context.Context, _ int64, state model.RunState, _ string, _ time.Time) error {
	s.finished = true
	s.state = state
	return nil
}

func TestCancellationRemainsClassifiableWhenRequeuePersistenceFails(t *testing.T) {
	store := &cancellationFailureStore{}
	err := HandleRunFailure(context.Background(), store, 88, context.Canceled, time.Date(2026, 8, 23, 8, 0, 0, 0, time.UTC))
	if store.finished || store.state == model.RunFailed {
		t.Fatal("canceled automation was persisted as a failed terminal run")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("combined failure no longer exposes cancellation: %v", err)
	}
}
