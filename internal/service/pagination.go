package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
)

type PageRequest struct {
	Limit  int
	Offset int
	Query  string
}

func (p PageRequest) Normalize() (PageRequest, error) {
	if p.Limit < 0 || p.Offset < 0 || p.Limit > 200 {
		return p, model.ErrInvalid
	}
	if p.Limit == 0 {
		p.Limit = 50
	}
	p.Query = strings.TrimSpace(p.Query)
	return p, nil
}

type Page[T any] struct {
	Items  []T `json:"items"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

func Paginate[T any](values []T, request PageRequest) (Page[T], error) {
	request, err := request.Normalize()
	if err != nil {
		return Page[T]{}, err
	}
	start := request.Offset
	if start > len(values) {
		start = len(values)
	}
	end := start + request.Limit
	if end > len(values) {
		end = len(values)
	}
	items := append([]T(nil), values[start:end]...)
	return Page[T]{Items: items, Total: len(values), Limit: request.Limit, Offset: request.Offset}, nil
}

type DeviceIndex struct {
	ByID       map[int64]model.Device
	ByExternal map[string]int64
}

func BuildDeviceIndex(devices []model.Device) (DeviceIndex, error) {
	index := DeviceIndex{ByID: make(map[int64]model.Device, len(devices)), ByExternal: make(map[string]int64, len(devices))}
	for _, device := range devices {
		if device.ID <= 0 || strings.TrimSpace(device.ExternalID) == "" {
			return DeviceIndex{}, errors.New("device identity is required")
		}
		key := strings.ToLower(strings.TrimSpace(device.ExternalID))
		if _, exists := index.ByExternal[key]; exists {
			return DeviceIndex{}, fmt.Errorf("duplicate external id %q", key)
		}
		index.ByID[device.ID] = device.Clone()
		index.ByExternal[key] = device.ID
	}
	return index, nil
}

func (i DeviceIndex) Get(id int64) (model.Device, bool) {
	d, ok := i.ByID[id]
	return d.Clone(), ok
}

func FilterDevices(ctx context.Context, devices []model.Device, query string) ([]model.Device, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	query = strings.ToLower(strings.TrimSpace(query))
	out := make([]model.Device, 0, len(devices))
	for _, device := range devices {
		if query == "" || strings.Contains(strings.ToLower(device.ExternalID), query) || strings.Contains(strings.ToLower(string(device.Kind)), query) {
			out = append(out, device.Clone())
		}
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].ID < out[b].ID })
	return out, nil
}
