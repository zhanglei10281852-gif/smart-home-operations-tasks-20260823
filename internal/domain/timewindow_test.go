package domain

import (
	"testing"
	"time"
)

func TestTimeWindowContainsAndMerge(t *testing.T) {
	zone := time.FixedZone("home", 8*60*60)
	base := time.Date(2026, 8, 23, 10, 0, 0, 0, zone)
	w := TimeWindow{Start: base, End: base.Add(time.Hour), Zone: zone}
	if err := w.Validate(2 * time.Hour); err != nil || !w.Contains(base.Add(30*time.Minute)) || w.Contains(base.Add(time.Hour)) {
		t.Fatalf("window=%+v err=%v", w, err)
	}
	merged, err := MergeWindows([]TimeWindow{{Start: base.Add(time.Hour), End: base.Add(2 * time.Hour), Zone: zone}, {Start: base, End: base.Add(90 * time.Minute), Zone: zone}})
	if err != nil || len(merged) != 1 || !merged[0].End.Equal(base.Add(2*time.Hour)) {
		t.Fatalf("merged=%+v err=%v", merged, err)
	}
}

func TestTimeWindowRejectsInvalid(t *testing.T) {
	base := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)
	if err := (TimeWindow{Start: base, End: base, Zone: time.UTC}).Validate(time.Hour); err == nil {
		t.Fatal("zero duration accepted")
	}
	if _, err := MergeWindows([]TimeWindow{{Start: base, End: base.Add(time.Hour)}}); err == nil {
		t.Fatal("window without zone accepted")
	}
}
