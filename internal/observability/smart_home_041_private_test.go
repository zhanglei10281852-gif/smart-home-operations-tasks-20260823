package observability

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestConcurrentMetricExportProducesCompleteStableText(t *testing.T) {
	registry := NewRegistry()
	const metrics = 96
	for i := 0; i < metrics; i++ {
		registry.Counter(fmt.Sprintf("home_metric_%03d", i), nil).Add(int64(i + 1))
	}
	exported, err := registry.Export(context.Background(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(exported), "\n")
	if len(lines) != metrics {
		t.Fatalf("export contained %d lines, want %d", len(lines), metrics)
	}
	for i, line := range lines {
		want := fmt.Sprintf("home_metric_%03d %d", i, i+1)
		if line != want {
			t.Fatalf("metric line %d=%q want=%q", i, line, want)
		}
	}
}
