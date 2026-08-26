package domain

import (
	"fmt"
	"sync"
	"time"
)

type Counter struct {
	mu     sync.Mutex
	values map[string]int64
}

func NewCounter() *Counter { return &Counter{values: map[string]int64{}} }
func (c *Counter) Add(name string, delta int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values[name] += delta
}
func (c *Counter) Get(name string) int64 { c.mu.Lock(); defer c.mu.Unlock(); return c.values[name] }

type Timer struct {
	Started time.Time
	Elapsed time.Duration
}

func StartTimer() Timer                            { return Timer{Started: time.Now()} }
func (t *Timer) Stop()                             { t.Elapsed = time.Since(t.Started) }
func FormatMetric(name string, value int64) string { return fmt.Sprintf("%s %d", name, value) }
