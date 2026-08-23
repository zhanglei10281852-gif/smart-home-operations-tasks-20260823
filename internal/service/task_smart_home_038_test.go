package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zhanglei10281852-gif/smart-home-operations/internal/domain"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
)

type task038DeviceRepo struct {
	devices map[int64]model.Device
}

func (r *task038DeviceRepo) CreateDevice(context.Context, model.Device, []string) (model.Device, error) {
	return model.Device{}, errors.New("not used")
}
func (r *task038DeviceRepo) GetDevice(_ context.Context, id int64) (model.Device, error) {
	device, ok := r.devices[id]
	if !ok {
		return model.Device{}, model.ErrNotFound
	}
	return device, nil
}
func (r *task038DeviceRepo) TransitionDevice(context.Context, int64, model.DeviceState, model.DeviceState, int64) error {
	return errors.New("not used")
}
func (r *task038DeviceRepo) TouchDevice(context.Context, int64, time.Time) error {
	return errors.New("not used")
}

func TestCancelledBatchJoinsWorkersBeforeReturningResults(t *testing.T) {
	repository := &task038DeviceRepo{devices: map[int64]model.Device{
		11: {ID: 11, Kind: model.KindLight, State: model.DeviceEnabled},
		22: {ID: 22, Kind: model.KindLight, State: model.DeviceEnabled},
	}}
	service := &BatchService{Devices: &DeviceService{Repo: repository}}
	commands := []BatchCommand{
		{DeviceID: 11, Action: "on", RequestID: "batch-11"},
		{DeviceID: 22, Action: "off", RequestID: "batch-22"},
	}
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan int64, len(commands))
	release := make(chan struct{})
	finished := make(chan int64, len(commands))
	type outcome struct {
		results []domain.BatchResult
		err     error
	}
	returned := make(chan outcome, 1)
	go func() {
		results, err := service.Execute(ctx, commands, func(_ context.Context, command BatchCommand) error {
			started <- command.DeviceID
			<-release
			finished <- command.DeviceID
			return nil
		})
		returned <- outcome{results: results, err: err}
	}()
	for range commands {
		<-started
	}
	cancel()

	var result outcome
	returnedEarly := false
	waitForOwnership := time.NewTimer(200 * time.Millisecond)
	select {
	case result = <-returned:
		returnedEarly = true
	case <-waitForOwnership.C:
	}
	if !waitForOwnership.Stop() {
		select {
		case <-waitForOwnership.C:
		default:
		}
	}
	close(release)
	if !returnedEarly {
		result = <-returned
	}
	for range commands {
		<-finished
	}

	if returnedEarly {
		t.Fatalf("batch returned before dispatch workers stopped: results=%+v err=%v", result.results, result.err)
	}
	if !errors.Is(result.err, context.Canceled) {
		t.Fatalf("cancelled batch error=%v", result.err)
	}
	if len(result.results) != len(commands) {
		t.Fatalf("cancelled batch result count=%d", len(result.results))
	}
	for index, deviceID := range []int64{11, 22} {
		if result.results[index].DeviceID != deviceID || !result.results[index].Accepted || result.results[index].Error != nil {
			t.Fatalf("cancelled batch result[%d]=%+v", index, result.results[index])
		}
	}
}
