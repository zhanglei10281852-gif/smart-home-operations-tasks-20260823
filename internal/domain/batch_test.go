package domain

import (
	"context"
	"errors"
	"sort"
	"sync/atomic"
	"testing"
	"time"
)

func TestBatchValidationRejectsBadInputs(t *testing.T) {
	cases := [][]BatchItem{nil, {}, {{DeviceID: 0, Action: "on"}}, {{DeviceID: 1, Action: ""}}, {{DeviceID: 1, Action: "on"}, {DeviceID: 1, Action: "off"}}}
	for _, items := range cases {
		if err := ValidateBatch(items); err == nil {
			t.Fatalf("accepted invalid batch: %#v", items)
		}
	}
	valid := make([]BatchItem, 100)
	for i := range valid {
		valid[i] = BatchItem{DeviceID: int64(i + 1), Action: "on"}
	}
	if err := ValidateBatch(valid); err != nil {
		t.Fatal(err)
	}
	valid = append(valid, BatchItem{DeviceID: 101, Action: "on"})
	if err := ValidateBatch(valid); err == nil {
		t.Fatal("batch above limit accepted")
	}
}

func TestExecuteBatchPreservesOrderAndErrors(t *testing.T) {
	items := []BatchItem{{DeviceID: 3, Action: "on"}, {DeviceID: 1, Action: "off"}, {DeviceID: 2, Action: "on"}}
	var calls atomic.Int32
	results, err := ExecuteBatch(context.Background(), items, func(_ context.Context, item BatchItem) error {
		calls.Add(1)
		if item.DeviceID == 1 {
			return errors.New("device unavailable")
		}
		return nil
	})
	if !errors.Is(err, errors.New("device unavailable")) && err == nil {
		t.Fatal("first error was lost")
	}
	if calls.Load() != 3 || len(results) != len(items) {
		t.Fatalf("calls=%d results=%d", calls.Load(), len(results))
	}
	if results[0].DeviceID != 3 || !results[0].Accepted || results[1].Accepted {
		t.Fatalf("results=%+v", results)
	}
	ordered := SortBatchResults(results)
	if !sort.SliceIsSorted(ordered, func(i, j int) bool { return ordered[i].DeviceID < ordered[j].DeviceID }) {
		t.Fatalf("not sorted: %+v", ordered)
	}
}

func TestExecuteBatchCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	items := []BatchItem{{DeviceID: 1, Action: "on"}}
	called := false
	results, err := ExecuteBatch(ctx, items, func(context.Context, BatchItem) error { called = true; return nil })
	if !errors.Is(err, context.Canceled) || called || len(results) != 1 || results[0].Accepted {
		t.Fatalf("err=%v called=%v results=%+v", err, called, results)
	}
}

func TestQuotaAndSafeAverage(t *testing.T) {
	now := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)
	q := Quota{Name: "commands", Limit: 5, Used: 2, ResetAt: now.Add(time.Hour)}
	if err := q.Validate(now); err != nil {
		t.Fatal(err)
	}
	reserved, err := q.Reserve(2)
	if err != nil || reserved.Remaining() != 1 {
		t.Fatalf("reserved=%+v err=%v", reserved, err)
	}
	if _, err := reserved.Reserve(2); err == nil {
		t.Fatal("quota overflow accepted")
	}
	if _, err := SafeAverage([]float64{1, 3, 5}); err != nil {
		t.Fatal(err)
	}
	if _, err := SafeAverage(nil); err == nil {
		t.Fatal("empty average accepted")
	}
}

func TestQuotaValidationBoundaries(t *testing.T) {
	now := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)
	invalid := []Quota{{Limit: 1, ResetAt: now.Add(time.Hour)}, {Name: "x", Used: 2, Limit: 1, ResetAt: now.Add(time.Hour)}, {Name: "x", Limit: 1, ResetAt: now}}
	for _, quota := range invalid {
		if err := quota.Validate(now); err == nil {
			t.Fatalf("invalid quota accepted: %+v", quota)
		}
	}
	hard := Quota{Name: "hard", Limit: 3, Used: 1, ResetAt: now.Add(time.Hour), HardBlock: true}
	if _, err := hard.Reserve(1); err == nil {
		t.Fatal("hard-block quota reserved")
	}
}
