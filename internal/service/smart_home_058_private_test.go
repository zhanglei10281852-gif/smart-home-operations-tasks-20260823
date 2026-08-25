package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zhanglei10281852-gif/smart-home-operations/internal/domain"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
)

type cancellationExecutor058 struct {
	started chan struct{}
}

func (e *cancellationExecutor058) Execute(ctx context.Context, _ domain.Command) (domain.CommandResult, error) {
	close(e.started)
	<-ctx.Done()
	return domain.CommandResult{}, ctx.Err()
}

func TestCanceledDeviceCommandStopsActiveExecution(t *testing.T) {
	executor := &cancellationExecutor058{started: make(chan struct{})}
	commands := NewCommands(executor)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := commands.Dispatch(ctx, domain.Command{
			DeviceID:       58,
			Kind:           model.KindLight,
			Action:         "on",
			IdempotencyKey: "command-058",
		})
		result <- err
	}()
	<-executor.started
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("dispatch err=%v want cancellation", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("device command remained active after caller cancellation")
	}
}
