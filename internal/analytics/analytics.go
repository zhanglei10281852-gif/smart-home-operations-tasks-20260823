package analytics

import (
	"math"
	"sort"
	"time"
)

type Sample struct {
	DeviceID    int64
	At          time.Time
	Watts       float64
	Temperature *float64
}
type Bucket struct {
	Start, End         time.Time
	Samples            int
	WattHours          float64
	AverageWatts       float64
	MinWatts, MaxWatts float64
}
type Summary struct {
	DeviceCount, Samples               int
	WattHours, PeakWatts, AverageWatts float64
	First, Last                        time.Time
}

func Summarize(samples []Sample) Summary {
	summary := Summary{}
	if len(samples) == 0 {
		return summary
	}
	devices := map[int64]struct{}{}
	ordered := append([]Sample(nil), samples...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].At.Before(ordered[j].At) })
	summary.First = ordered[0].At
	summary.Last = ordered[len(ordered)-1].At
	for _, sample := range ordered {
		devices[sample.DeviceID] = struct{}{}
		summary.Samples++
		summary.AverageWatts += sample.Watts
		if sample.Watts > summary.PeakWatts {
			summary.PeakWatts = sample.Watts
		}
	}
	summary.DeviceCount = len(devices)
	summary.AverageWatts /= float64(summary.Samples)
	summary.WattHours = EnergyWh(ordered)
	return summary
}
func EnergyWh(samples []Sample) float64 {
	if len(samples) < 2 {
		return 0
	}
	ordered := append([]Sample(nil), samples...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].At.Before(ordered[j].At) })
	total := 0.0
	for i := 1; i < len(ordered); i++ {
		duration := ordered[i].At.Sub(ordered[i-1].At).Hours()
		if duration < 0 || duration > 24 {
			continue
		}
		total += (ordered[i-1].Watts + ordered[i].Watts) / 2 * duration
	}
	return total
}
func Bucketize(samples []Sample, start, end time.Time, step time.Duration) []Bucket {
	if step <= 0 || !start.Before(end) {
		return nil
	}
	count := int(math.Ceil(end.Sub(start).Seconds() / step.Seconds()))
	out := make([]Bucket, count)
	for i := range out {
		out[i].Start = start.Add(time.Duration(i) * step)
		out[i].End = out[i].Start.Add(step)
	}
	for _, sample := range samples {
		if sample.At.Before(start) || !sample.At.Before(end) {
			continue
		}
		idx := int(sample.At.Sub(start) / step)
		if idx < 0 || idx >= len(out) {
			continue
		}
		bucket := &out[idx]
		bucket.Samples++
		bucket.AverageWatts += sample.Watts
		if bucket.Samples == 1 || sample.Watts < bucket.MinWatts {
			bucket.MinWatts = sample.Watts
		}
		if sample.Watts > bucket.MaxWatts {
			bucket.MaxWatts = sample.Watts
		}
	}
	for i := range out {
		if out[i].Samples > 0 {
			out[i].AverageWatts /= float64(out[i].Samples)
			out[i].WattHours = out[i].AverageWatts * out[i].End.Sub(out[i].Start).Hours()
		}
	}
	return out
}
func Percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	if p < 0 {
		p = 0
	}
	if p > 1 {
		p = 1
	}
	ordered := append([]float64(nil), values...)
	sort.Float64s(ordered)
	index := int(math.Round(p * float64(len(ordered)-1)))
	return ordered[index]
}
