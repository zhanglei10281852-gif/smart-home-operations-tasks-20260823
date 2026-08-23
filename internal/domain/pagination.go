package domain

import (
	"fmt"
	"strings"
)

type Page struct {
	Limit, Offset int
	Sort, Query   string
}

func NormalizePage(p Page) Page {
	if p.Limit <= 0 || p.Limit > 200 {
		p.Limit = 50
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	p.Sort = strings.TrimSpace(p.Sort)
	p.Query = strings.TrimSpace(p.Query)
	return p
}
func ValidatePage(p Page) error {
	if p.Limit < 0 || p.Limit > 200 || p.Offset < 0 {
		return fmt.Errorf("invalid page: %w", fmt.Errorf("bounds"))
	}
	return nil
}

type Result[T any] struct {
	Items                []T
	Total, Limit, Offset int
}

func NewResult[T any](items []T, total int, p Page) Result[T] {
	p = NormalizePage(p)
	if items == nil {
		items = []T{}
	}
	return Result[T]{Items: items, Total: total, Limit: p.Limit, Offset: p.Offset}
}
