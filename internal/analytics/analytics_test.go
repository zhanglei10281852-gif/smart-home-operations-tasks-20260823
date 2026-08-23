package analytics

import (
	"math"
	"testing"
	"time"
)

func TestSummarize(t *testing.T) {
	base := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)
	rows := []Sample{{DeviceID: 1, At: base, Watts: 10}, {DeviceID: 1, At: base.Add(time.Hour), Watts: 20}, {DeviceID: 2, At: base.Add(2 * time.Hour), Watts: 30}}
	s := Summarize(rows)
	if s.DeviceCount != 2 || s.Samples != 3 || s.PeakWatts != 30 || s.AverageWatts != 20 {
		t.Fatalf("summary=%+v", s)
	}
	if math.Abs(s.WattHours-40) > 0.001 {
		t.Fatalf("energy=%v", s.WattHours)
	}
}
func TestBucketize(t *testing.T) {
	base := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)
	rows := []Sample{{At: base.Add(5 * time.Minute), Watts: 10}, {At: base.Add(35 * time.Minute), Watts: 20}, {At: base.Add(65 * time.Minute), Watts: 30}}
	b := Bucketize(rows, base, base.Add(2*time.Hour), time.Hour)
	if len(b) != 2 || b[0].Samples != 2 || b[1].Samples != 1 {
		t.Fatalf("buckets=%+v", b)
	}
}
func TestPercentile(t *testing.T) {
	if Percentile([]float64{1, 2, 3, 4, 5}, 0.5) != 3 {
		t.Fatal("median incorrect")
	}
	if Percentile(nil, .5) != 0 {
		t.Fatal("empty percentile incorrect")
	}
	if Percentile([]float64{5, 1}, 0) != 1 || Percentile([]float64{5, 1}, 1) != 5 {
		t.Fatal("edge percentile incorrect")
	}
}
