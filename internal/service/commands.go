package service

import (
	"context"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/domain"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
)

type CommandService struct{ Executor domain.CommandExecutor }

func NewCommands(e domain.CommandExecutor) *CommandService { return &CommandService{Executor: e} }
func (s *CommandService) Dispatch(ctx context.Context, c domain.Command) (domain.CommandResult, error) {
	executionCtx, cancel := domain.CommandExecutionContext(ctx)
	defer cancel()
	return domain.Dispatch(executionCtx, s.Executor, c)
}

type DeviceCommandExecutor struct {
	Commands []domain.Command
	Err      error
}

func (e *DeviceCommandExecutor) Execute(_ context.Context, c domain.Command) (domain.CommandResult, error) {
	if e.Err != nil {
		return domain.CommandResult{}, e.Err
	}
	e.Commands = append(e.Commands, c)
	return domain.CommandResult{Accepted: true, Message: "accepted"}, nil
}

var _ = model.ErrBusy
