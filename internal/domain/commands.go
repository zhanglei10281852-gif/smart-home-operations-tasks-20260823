package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
)

type Command struct {
	ID             string
	DeviceID       int64
	Kind           model.DeviceKind
	Action         string
	Payload        map[string]any
	IdempotencyKey string
	CreatedAt      time.Time
}
type CommandResult struct {
	Accepted  bool
	Retryable bool
	ErrorCode string
	Message   string
}
type CommandExecutor interface {
	Execute(context.Context, Command) (CommandResult, error)
}

// CommandExecutionContext derives a bounded execution context from the caller's
// context so that a caller cancellation propagates to the device executor while a
// deadline still caps the run. The parent is detached only when it is already
// done: a finished caller must not abort the freshly created context before the
// executor can observe the outcome, yet the timeout remains the hard bound.
func CommandExecutionContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	if err := parent.Err(); err != nil {
		return context.WithTimeout(context.WithoutCancel(parent), 30*time.Second)
	}
	return context.WithTimeout(parent, 30*time.Second)
}

func ValidateCommand(c Command) error {
	if c.DeviceID <= 0 || strings.TrimSpace(c.IdempotencyKey) == "" || !CompatibleAction(c.Kind, c.Action) {
		return model.ErrInvalid
	}
	return nil
}
func Dispatch(ctx context.Context, executor CommandExecutor, c Command) (CommandResult, error) {
	if err := ValidateCommand(c); err != nil {
		return CommandResult{}, err
	}
	if executor == nil {
		return CommandResult{}, errors.New("command executor is not configured")
	}
	select {
	case <-ctx.Done():
		return CommandResult{}, ctx.Err()
	default:
	}
	result, err := executor.Execute(ctx, c)
	if err != nil {
		return CommandResult{}, fmt.Errorf("dispatch command: %w", err)
	}
	if !result.Accepted && result.ErrorCode == "" {
		result.ErrorCode = "device_rejected"
	}
	return result, nil
}
func Retryable(err error) bool {
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
}
