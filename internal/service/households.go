package service

import (
	"context"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/domain"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"strings"
	"time"
)

type HouseholdService struct {
	Repo interface {
		CreateHousehold(context.Context, string, string, int64) (model.Household, error)
		CreateHouseholdWithOwner(context.Context, string, string, int64, string, string) (model.Household, model.Member, error)
		AddMember(context.Context, int64, string, string, model.Role) (model.Member, error)
	}
}

func NewHouseholds(r interface {
	CreateHousehold(context.Context, string, string, int64) (model.Household, error)
	CreateHouseholdWithOwner(context.Context, string, string, int64, string, string) (model.Household, model.Member, error)
	AddMember(context.Context, int64, string, string, model.Role) (model.Member, error)
}) *HouseholdService {
	return &HouseholdService{Repo: r}
}
func (s *HouseholdService) Onboard(ctx context.Context, name, timezone string, budget int64, email, password string) (model.Household, model.Member, error) {
	name = strings.TrimSpace(name)
	timezone = strings.TrimSpace(timezone)
	email = strings.TrimSpace(strings.ToLower(email))
	if name == "" || budget < 0 || len(password) < 10 || domain.ValidateEmail(email) != nil {
		return model.Household{}, model.Member{}, model.ErrInvalid
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return model.Household{}, model.Member{}, model.ErrInvalid
	}
	hash, err := hashPassword(password)
	if err != nil {
		return model.Household{}, model.Member{}, err
	}
	return s.Repo.CreateHouseholdWithOwner(ctx, name, timezone, budget, email, hash)
}
func (s *HouseholdService) Create(ctx context.Context, name, timezone string, budget int64) (model.Household, error) {
	name = strings.TrimSpace(name)
	timezone = strings.TrimSpace(timezone)
	if name == "" || budget < 0 {
		return model.Household{}, model.ErrInvalid
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return model.Household{}, model.ErrInvalid
	}
	return s.Repo.CreateHousehold(ctx, name, timezone, budget)
}
func (s *HouseholdService) AddMember(ctx context.Context, h int64, email, password string, role model.Role) (model.Member, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if h <= 0 || len(password) < 10 || domain.ValidateEmail(email) != nil {
		return model.Member{}, model.ErrInvalid
	}
	if _, ok := map[model.Role]struct{}{model.RoleOwner: {}, model.RoleOperator: {}, model.RoleViewer: {}}[role]; !ok {
		return model.Member{}, model.ErrInvalid
	}
	hash, err := hashPassword(password)
	if err != nil {
		return model.Member{}, err
	}
	return s.Repo.AddMember(ctx, h, email, hash, role)
}
