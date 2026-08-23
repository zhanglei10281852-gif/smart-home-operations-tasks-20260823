package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zhanglei10281852-gif/smart-home-operations/internal/domain"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
)

func TestDeviceCommandValidation(t *testing.T) {
	device := model.Device{ID: 1, Kind: model.KindLight, State: model.DeviceEnabled}
	for _, action := range []string{"on", "off"} {
		if err := ValidateDeviceCommand(device, action); err != nil {
			t.Fatalf("light action %q rejected: %v", action, err)
		}
	}
	for _, tc := range []struct {
		device model.Device
		action string
	}{
		{model.Device{ID: 1, Kind: model.KindLight, State: model.DeviceDisabled}, "on"},
		{model.Device{ID: 1, Kind: model.KindThermostat, State: model.DeviceEnabled}, "on"},
		{model.Device{ID: 1, Kind: model.KindSensor, State: model.DeviceEnabled}, "read"},
	} {
		if err := ValidateDeviceCommand(tc.device, tc.action); err == nil {
			t.Fatalf("invalid command accepted: %+v", tc)
		}
	}
}

func TestBatchServiceDispatchAndAudit(t *testing.T) {
	f := &fakeRepo{device: model.Device{ID: 4, Kind: model.KindLight, State: model.DeviceEnabled}}
	devices := NewDevices(f, model.RealClock{})
	audit := NewAudit(f)
	s := NewBatch(devices, audit, model.RealClock{})
	commands := []BatchCommand{{DeviceID: 4, Action: "on"}}
	called := 0
	results, err := s.Execute(context.Background(), commands, func(_ context.Context, command BatchCommand) error {
		called++
		if command.Action != "on" {
			t.Fatal("wrong action")
		}
		return nil
	})
	if err != nil || called != 1 || len(results) != 1 || !results[0].Accepted {
		t.Fatalf("err=%v called=%d results=%+v", err, called, results)
	}
	if err := s.AuditResult(context.Background(), 1, nil, "req-1", results[0]); err != nil {
		t.Fatal(err)
	}
}

func TestBatchServiceRejectsUnavailableDevice(t *testing.T) {
	f := &fakeRepo{device: model.Device{ID: 4, Kind: model.KindLight, State: model.DeviceDisabled}}
	s := NewBatch(NewDevices(f, model.RealClock{}), NewAudit(f), model.RealClock{})
	if _, err := s.Execute(context.Background(), []BatchCommand{{DeviceID: 4, Action: "on"}}, func(context.Context, BatchCommand) error { return nil }); err == nil {
		t.Fatal("disabled device dispatched")
	}
}

func TestPaginationAndIndexIsolation(t *testing.T) {
	devices := []model.Device{{ID: 2, ExternalID: "Kitchen-Lamp", Kind: model.KindLight}, {ID: 1, ExternalID: "Hall-Thermostat", Kind: model.KindThermostat}}
	page, err := Paginate(devices, PageRequest{Limit: 1, Offset: 1})
	if err != nil || page.Total != 2 || len(page.Items) != 1 || page.Items[0].ID != 1 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	index, err := BuildDeviceIndex(devices)
	if err != nil {
		t.Fatal(err)
	}
	value, ok := index.Get(2)
	if !ok || value.ExternalID != "Kitchen-Lamp" {
		t.Fatalf("value=%+v ok=%v", value, ok)
	}
	value.ExternalID = "mutated"
	again, _ := index.Get(2)
	if again.ExternalID == "mutated" {
		t.Fatal("index leaked mutable device")
	}
	filtered, err := FilterDevices(context.Background(), devices, "thermo")
	if err != nil || len(filtered) != 1 || filtered[0].ID != 1 {
		t.Fatalf("filtered=%+v err=%v", filtered, err)
	}
}

func TestDispatchRecordValidation(t *testing.T) {
	now := time.Now().UTC()
	records := []DispatchRecord{{RequestID: "r", DeviceID: 1, Action: "on", State: "accepted", At: now}}
	if err := ValidateDispatchRecords(records); err != nil {
		t.Fatal(err)
	}
	records = append(records, records[0])
	if err := ValidateDispatchRecords(records); err == nil {
		t.Fatal("duplicate record accepted")
	}
	if !errors.Is(model.ErrInvalid, model.ErrInvalid) {
		t.Fatal("sentinel sanity check failed")
	}
	_ = domain.BatchResult{}
}

func TestPaginationRejectsInvalidRequests(t *testing.T) {
	for _, request := range []PageRequest{{Limit: -1}, {Offset: -1}, {Limit: 201}} {
		if _, err := request.Normalize(); err == nil {
			t.Fatalf("invalid page accepted: %+v", request)
		}
	}
	page, err := Paginate([]int{1, 2, 3}, PageRequest{Limit: 2, Offset: 20})
	if err != nil || len(page.Items) != 0 || page.Total != 3 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
}
