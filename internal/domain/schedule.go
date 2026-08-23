package domain

import (
	"errors"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"sort"
	"time"
)

type ScheduleWindow struct {
	DeviceID   int64
	Start, End time.Time
	Watts      float64
}

func (w ScheduleWindow) Valid() error {
	if w.DeviceID <= 0 || !w.Start.Before(w.End) || w.Watts < 0 {
		return model.ErrInvalid
	}
	return nil
}
func Overlap(a, b ScheduleWindow) bool {
	return a.DeviceID == b.DeviceID && a.Start.Before(b.End) && b.Start.Before(a.End)
}
func Conflicts(rows []ScheduleWindow) []ScheduleWindow {
	out := []ScheduleWindow{}
	copyRows := append([]ScheduleWindow(nil), rows...)
	sort.SliceStable(copyRows, func(i, j int) bool { return copyRows[i].Start.Before(copyRows[j].Start) })
	for i := range copyRows {
		for j := i + 1; j < len(copyRows); j++ {
			if copyRows[j].Start.After(copyRows[i].End) {
				break
			}
			if Overlap(copyRows[i], copyRows[j]) {
				out = append(out, copyRows[j])
			}
		}
	}
	return out
}
func Capacity(rows []ScheduleWindow, limit float64) error {
	if limit < 0 {
		return model.ErrInvalid
	}
	byTime := map[time.Time]float64{}
	points := []time.Time{}
	for _, row := range rows {
		if err := row.Valid(); err != nil {
			return err
		}
		points = append(points, row.Start, row.End)
	}
	sort.Slice(points, func(i, j int) bool { return points[i].Before(points[j]) })
	for _, point := range points {
		sum := 0.0
		for _, row := range rows {
			if !point.Before(row.Start) && point.Before(row.End) {
				sum += row.Watts
			}
		}
		byTime[point] = sum
		if sum > limit {
			return errors.New("schedule exceeds household capacity")
		}
	}
	_ = byTime
	return nil
}
