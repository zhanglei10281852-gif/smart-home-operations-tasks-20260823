package domain

import (
	"errors"
	"sort"
	"time"
)

type TimeWindow struct {
	Start time.Time
	End   time.Time
	Zone  *time.Location
}

func (w TimeWindow) Validate(max time.Duration) error {
	if w.Start.IsZero() || w.End.IsZero() || w.Zone == nil || !w.Start.Before(w.End) || max <= 0 || w.End.Sub(w.Start) > max {
		return errors.New("invalid time window")
	}
	return nil
}

func (w TimeWindow) Contains(at time.Time) bool {
	if w.Start.IsZero() || w.End.IsZero() || w.Zone == nil {
		return false
	}
	at = at.In(w.Zone)
	return !at.Before(w.Start.In(w.Zone)) && at.Before(w.End.In(w.Zone))
}

func MergeWindows(windows []TimeWindow) ([]TimeWindow, error) {
	copyWindows := append([]TimeWindow(nil), windows...)
	for _, window := range copyWindows {
		if err := window.Validate(366 * 24 * time.Hour); err != nil {
			return nil, err
		}
	}
	sort.Slice(copyWindows, func(i, j int) bool { return copyWindows[i].Start.Before(copyWindows[j].Start) })
	merged := make([]TimeWindow, 0, len(copyWindows))
	for _, window := range copyWindows {
		if len(merged) == 0 {
			merged = append(merged, window)
			continue
		}
		last := &merged[len(merged)-1]
		if !window.Start.After(last.End) {
			if window.End.After(last.End) {
				last.End = window.End
			}
			continue
		}
		merged = append(merged, window)
	}
	return merged, nil
}
