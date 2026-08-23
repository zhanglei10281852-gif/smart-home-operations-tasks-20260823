package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/repo"
)

type AuthService struct {
	Repo interface {
		AddMember(context.Context, int64, string, string, model.Role) (model.Member, error)
		FindMember(context.Context, int64, string) (model.Member, string, error)
		CreateSession(context.Context, model.Session) error
		GetSession(context.Context, string) (model.Session, error)
		MemberByID(context.Context, int64) (model.Member, error)
		RevokeSession(context.Context, string, time.Time) error
	}
	Clock      model.Clock
	SessionTTL time.Duration
}

func NewAuth(r interface {
	AddMember(context.Context, int64, string, string, model.Role) (model.Member, error)
	FindMember(context.Context, int64, string) (model.Member, string, error)
	CreateSession(context.Context, model.Session) error
	GetSession(context.Context, string) (model.Session, error)
	MemberByID(context.Context, int64) (model.Member, error)
	RevokeSession(context.Context, string, time.Time) error
}, clock model.Clock) *AuthService {
	return &AuthService{Repo: r, Clock: clock, SessionTTL: 12 * time.Hour}
}
func passwordDigest(password string) string {
	sum := sha256.Sum256([]byte(password))
	return hex.EncodeToString(sum[:])
}
func (s *AuthService) Register(ctx context.Context, householdID int64, email, password string, role model.Role) (model.Member, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if householdID <= 0 || email == "" || len(password) < 10 {
		return model.Member{}, model.ErrInvalid
	}
	return s.Repo.AddMember(ctx, householdID, email, passwordDigest(password), role)
}
func (s *AuthService) Login(ctx context.Context, householdID int64, email, password string) (model.Session, model.Member, error) {
	m, hash, err := s.Repo.FindMember(ctx, householdID, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		return model.Session{}, model.Member{}, model.ErrForbidden
	}
	if !m.Active || hash != passwordDigest(password) {
		return model.Session{}, model.Member{}, model.ErrForbidden
	}
	now := s.Clock.Now()
	sess := model.Session{ID: uuid.NewString(), MemberID: m.ID, ExpiresAt: now.Add(s.SessionTTL), CreatedAt: now}
	if err := s.Repo.CreateSession(ctx, sess); err != nil {
		return model.Session{}, model.Member{}, repo.Wrap("create session", err)
	}
	return sess, m, nil
}
func (s *AuthService) Authorize(ctx context.Context, sessionID string, required model.Role) (model.Member, error) {
	if strings.TrimSpace(sessionID) == "" {
		return model.Member{}, model.ErrForbidden
	}
	sess, err := s.Repo.GetSession(ctx, sessionID)
	if err != nil {
		return model.Member{}, model.ErrForbidden
	}
	if sess.RevokedAt != nil || !s.Clock.Now().Before(sess.ExpiresAt) {
		return model.Member{}, model.ErrForbidden
	}
	// Member lookups are deliberately performed through the same repository boundary as all other requests.
	m, err := s.Repo.MemberByID(ctx, sess.MemberID)
	if err != nil || !m.Active {
		return model.Member{}, model.ErrForbidden
	}
	if err := EnsureRole(m.Role, required); err != nil {
		return model.Member{}, err
	}
	return m, nil
}
func (s *AuthService) Logout(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return model.ErrInvalid
	}
	return s.Repo.RevokeSession(ctx, sessionID, s.Clock.Now())
}
func EnsureRole(actual, required model.Role) error {
	rank := map[model.Role]int{model.RoleViewer: 1, model.RoleOperator: 2, model.RoleOwner: 3}
	if rank[actual] < rank[required] {
		return fmt.Errorf("%w: role %s requires %s", model.ErrForbidden, required, actual)
	}
	return nil
}
