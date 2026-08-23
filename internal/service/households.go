package service

import (
	"context"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"strings"
)

type HouseholdService struct {
	Repo interface {
		CreateHousehold(context.Context, string, string, int64) (model.Household, error)
		AddMember(context.Context, int64, string, string, model.Role) (model.Member, error)
	}
}

func NewHouseholds(r interface {
	CreateHousehold(context.Context, string, string, int64) (model.Household, error)
	AddMember(context.Context, int64, string, string, model.Role) (model.Member, error)
}) *HouseholdService {
	return &HouseholdService{Repo: r}
}
func (s *HouseholdService) Create(ctx context.Context, name, timezone string, budget int64) (model.Household, error) {
	name = strings.TrimSpace(name)
	timezone = strings.TrimSpace(timezone)
	if name == "" || timezone == "" || budget < 0 {
		return model.Household{}, model.ErrInvalid
	}
	return s.Repo.CreateHousehold(ctx, name, timezone, budget)
}
func (s *HouseholdService) AddMember(ctx context.Context, h int64, email, password string, role model.Role) (model.Member, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if h <= 0 || email == "" || len(password) < 10 {
		return model.Member{}, model.ErrInvalid
	}
	return s.Repo.AddMember(ctx, h, email, passwordDigest(password), role)
}
