package worker

import "time"

type Backoff struct{ Base, Max time.Duration }

func (b Backoff) Duration(attempt int) time.Duration {
	if attempt < 1 {
		return b.Base
	}
	d := b.Base
	for i := 1; i < attempt; i++ {
		if d >= b.Max/2 {
			return b.Max
		}
		d *= 2
	}
	if d > b.Max {
		return b.Max
	}
	return d
}
func Sleep(ctxDone <-chan struct{}, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctxDone:
		return false
	}
}
