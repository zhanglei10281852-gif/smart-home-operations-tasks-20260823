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
