package domain

import (
	"encoding/json"
	"fmt"
	"time"
)

type Event struct {
	Topic                        string
	HouseholdID                  int64
	ObjectType, ObjectID, Action string
	RequestID                    string
	Payload                      map[string]any
	At                           time.Time
}

func (e Event) Validate() error {
	if e.Topic == "" || e.HouseholdID <= 0 || e.ObjectType == "" || e.ObjectID == "" || e.Action == "" || e.RequestID == "" {
		return fmt.Errorf("event fields required")
	}
	return nil
}
func (e Event) JSON() ([]byte, error) {
	if err := e.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(e)
}

type EventConsumer interface{ Consume(Event) error }
type Fanout struct{ Consumers []EventConsumer }

func (f Fanout) Publish(e Event) error {
	if err := e.Validate(); err != nil {
		return err
	}
	return runConcurrentJoinAll(len(f.Consumers), func(index int) error {
		consumer := f.Consumers[index]
		if consumer == nil {
			return nil
		}
		return consumer.Consume(e)
	})
}
