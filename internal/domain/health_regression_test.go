package domain

import (
	"context"
	"strings"
	"testing"
)

func TestRunChecksReportsMissingCallback(t *testing.T) {
	results := RunChecks(context.Background(), []Check{{Name: "postgres"}})
	if len(results) != 1 || results[0].Healthy || !strings.Contains(results[0].Error, "not configured") {
		t.Fatalf("results=%+v", results)
	}
}
