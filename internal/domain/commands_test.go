package domain

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
)

func validCommand() Command {
	return Command{ID: "cmd", DeviceID: 1, Kind: model.KindLight, Action: "on", IdempotencyKey: "key"}
}

type recordingExecutor struct {
	started chan struct{}
	block   chan struct{}
	calls   int32
}

func (e *recordingExecutor) Execute(ctx context.Context, c Command) (CommandResult, error) {
	atomic.AddInt32(&e.calls, 1)
	if e.started != nil {
		select {
		case e.started <- struct{}{}:
		default:
		}
	}
	select {
	case <-ctx.Done():
		return CommandResult{}, ctx.Err()
	case <-e.block:
		return CommandResult{Accepted: true, Message: "accepted"}, nil
	}
}

func TestCommandExecutionContextPropagatesCallerCancellation(t *testing.T) {
	exec := &recordingExecutor{started: make(chan struct{}, 1), block: make(chan struct{})}

	caller, cancel := context.WithCancel(context.Background())
	go func() {
		<-exec.started
		cancel()
	}()

	_, err := Dispatch(caller, exec, validCommand())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v want context.Canceled", err)
	}

	select {
	case <-time.After(time.Second):
		t.Fatal("executor did not return after caller cancellation")
	default:
	}
	if atomic.LoadInt32(&exec.calls) != 1 {
		t.Fatalf("executor calls=%d want 1", exec.calls)
	}
}

func TestCommandExecutionContextRetainsBoundedTimeout(t *testing.T) {
	exec := &recordingExecutor{block: make(chan struct{})}

	ctx, cancel := CommandExecutionContext(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { _, err := Dispatch(ctx, exec, validCommand()); done <- err }()

	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("err=%v want context.DeadlineExceeded", err)
		}
	case <-time.After(35 * time.Second):
		t.Fatal("timeout did not bound execution")
	}
	if atomic.LoadInt32(&exec.calls) != 1 {
		t.Fatalf("executor calls=%d want 1", exec.calls)
	}
}

func TestCommandExecutionContextFinishedParentStillBounded(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	cancel()

	ctx, cancel2 := CommandExecutionContext(parent)
	defer cancel2()

	select {
	case <-ctx.Done():
		t.Fatalf("detached context must not be already done: %v", ctx.Err())
	default:
	}
	select {
	case <-time.After(50 * time.Millisecond):
	case <-ctx.Done():
		// An already-finished caller produces a detached, still-bounded context.
	}
}
