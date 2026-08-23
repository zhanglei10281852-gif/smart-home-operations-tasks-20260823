package domain

import (
	"encoding/json"
	"fmt"
	"time"
)

type AlertPolicy struct {
	Code      string
	Severity  string
	Cooldown  time.Duration
	Threshold float64
	Window    time.Duration
}
type AlertDecision struct {
	Open    bool
	Resolve bool
	Reason  string
}

func EvaluateAlert(policy AlertPolicy, value float64, last *time.Time, now time.Time, currentlyOpen bool) AlertDecision {
	if last != nil && now.Sub(*last) < policy.Cooldown {
		return AlertDecision{Reason: "cooldown"}
	}
	trigger := value >= policy.Threshold
	if trigger && !currentlyOpen {
		return AlertDecision{Open: true, Reason: fmt.Sprintf("%s threshold reached", policy.Code)}
	}
	if !trigger && currentlyOpen {
		return AlertDecision{Resolve: true, Reason: "value recovered"}
	}
	return AlertDecision{Reason: "unchanged"}
}
func EncodeDetails(values map[string]any) ([]byte, error) {
	if values == nil {
		values = map[string]any{}
	}
	return json.Marshal(values)
}
func ValidSeverity(v string) bool   { return v == "info" || v == "warning" || v == "critical" }
func ValidAlertState(v string) bool { return v == "open" || v == "acknowledged" || v == "resolved" }
func TransitionAlert(current, next string) bool {
	switch current {
	case "open":
		return next == "acknowledged" || next == "resolved"
	case "acknowledged":
		return next == "resolved"
	case "resolved":
		return false
	default:
		return false
	}
}
