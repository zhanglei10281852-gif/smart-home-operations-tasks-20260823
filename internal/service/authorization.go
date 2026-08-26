package service

import (
	"context"
	"fmt"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
)

type sessionCreator interface {
	CreateSession(context.Context, model.Session) error
}

func beginSessionWrite(ctx context.Context, store sessionCreator, session model.Session) <-chan error {
	done := make(chan error, 1)
	persistCtx := context.WithoutCancel(ctx)
	go func() {
		done <- store.CreateSession(persistCtx, session)
	}()
	return done
}

type AuthorizationService struct {
	Repo interface {
		MemberByID(context.Context, int64) (model.Member, error)
	}
}

func (a AuthorizationService) Require(ctx context.Context, memberID int64, required model.Role) (model.Member, error) {
	m, err := a.Repo.MemberByID(ctx, memberID)
	if err != nil {
		return model.Member{}, fmt.Errorf("authorize member: %w", err)
	}
	if !m.Active {
		return model.Member{}, model.ErrForbidden
	}
	if err := EnsureRole(m.Role, required); err != nil {
		return model.Member{}, err
	}
	return m, nil
}
func CanManageDevices(role model.Role) bool {
	return role == model.RoleOwner || role == model.RoleOperator
}
func CanViewReports(role model.Role) bool {
	return role == model.RoleOwner || role == model.RoleOperator || role == model.RoleViewer
}
