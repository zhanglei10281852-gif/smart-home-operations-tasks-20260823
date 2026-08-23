package observability

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type Counter struct {
	mu     sync.Mutex
	value  int64
	labels map[string]string
}

func NewCounter(labels map[string]string) *Counter {
	copyLabels := make(map[string]string, len(labels))
	for key, value := range labels {
		copyLabels[key] = value
	}
	return &Counter{labels: copyLabels}
}

func (c *Counter) Add(delta int64) {
	c.mu.Lock()
	c.value += delta
	c.mu.Unlock()
}

func (c *Counter) Value() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value
}

func (c *Counter) Snapshot() Metric {
	c.mu.Lock()
	defer c.mu.Unlock()
	labels := make(map[string]string, len(c.labels))
	for key, value := range c.labels {
		labels[key] = value
	}
	return Metric{Name: "counter", Value: c.value, Labels: labels}
}

type Metric struct {
	Name     string
	Value    int64
	Labels   map[string]string
	Observed time.Time
}

type Registry struct {
	mu       sync.RWMutex
	counters map[string]*Counter
}

func NewRegistry() *Registry { return &Registry{counters: make(map[string]*Counter)} }

func (r *Registry) Counter(name string, labels map[string]string) *Counter {
	name = strings.TrimSpace(name)
	r.mu.Lock()
	defer r.mu.Unlock()
	if c := r.counters[name]; c != nil {
		return c
	}
	c := NewCounter(labels)
	r.counters[name] = c
	return c
}

func (r *Registry) Snapshot(now time.Time) []Metric {
	r.mu.RLock()
	defer r.mu.RUnlock()
	keys := make([]string, 0, len(r.counters))
	for key := range r.counters {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]Metric, 0, len(keys))
	for _, key := range keys {
		metric := r.counters[key].Snapshot()
		metric.Name = key
		metric.Observed = now
		result = append(result, metric)
	}
	return result
}

func (r *Registry) Export(ctx context.Context, now time.Time) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	var b strings.Builder
	for _, metric := range r.Snapshot(now) {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "%s %d\n", metric.Name, metric.Value)
	}
	return b.String(), nil
}
