package domain

import (
	"errors"
	"math"
	"strings"
	"time"
)

type Quota struct {
	Name      string
	Limit     int
	Used      int
	ResetAt   time.Time
	HardBlock bool
}

func (q Quota) Validate(now time.Time) error {
	if strings.TrimSpace(q.Name) == "" || q.Limit < 0 || q.Used < 0 || q.Used > q.Limit || q.ResetAt.IsZero() {
		return errors.New("invalid quota")
	}
	if !q.ResetAt.After(now) {
		return errors.New("quota reset must be in the future")
	}
	return nil
}

func (q Quota) Remaining() int {
	if q.Used >= q.Limit {
		return 0
	}
	return q.Limit - q.Used
}

func (q Quota) Reserve(n int) (Quota, error) {
	if n <= 0 || n > q.Remaining() || (q.HardBlock && q.Used > 0) {
		return q, errors.New("quota exceeded")
	}
	q.Used += n
	return q, nil
}

func SafeAverage(values []float64) (float64, error) {
	if len(values) == 0 {
		return 0, errors.New("empty values")
	}
	var sum float64
	for _, v := range values {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, errors.New("non-finite value")
		}
		sum += v
	}
	return sum / float64(len(values)), nil
}
