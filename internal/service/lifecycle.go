package service

import (
	"context"
	"errors"
	"sync"
	"time"
)

type Lifecycle struct {
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
}

func StartLifecycle(parent context.Context, run func(context.Context)) *Lifecycle {
	ctx, cancel := context.WithCancel(parent)
	l := &Lifecycle{cancel: cancel, done: make(chan struct{})}
	go func() { defer close(l.done); run(ctx) }()
	return l
}
func (l *Lifecycle) Stop(timeout time.Duration) error {
	l.once.Do(l.cancel)
	select {
	case <-l.done:
		return nil
	case <-time.After(timeout):
		return errors.New("lifecycle stop timeout")
	}
}
