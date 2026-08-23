package service

import (
	"context"
	"time"
)

func lifecycleContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithCancel(context.WithoutCancel(parent))
}

type FixedClock struct{ Value time.Time }

func (c FixedClock) Now() time.Time { return c.Value }

type SequenceClock struct {
	Values []time.Time
	Index  int
}

func (c *SequenceClock) Now() time.Time {
	if len(c.Values) == 0 {
		return time.Time{}
	}
	if c.Index >= len(c.Values) {
		return c.Values[len(c.Values)-1]
	}
	v := c.Values[c.Index]
	c.Index++
	return v
}
