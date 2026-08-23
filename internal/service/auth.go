package service

import (
	"context"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/domain"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/repo"
)

const (
	passwordIterations = 210_000
	passwordSaltBytes  = 16
	passwordKeyBytes   = 32
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
func hashPassword(password string) (string, error) {
	salt := make([]byte, passwordSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, passwordIterations, passwordKeyBytes)
	if err != nil {
		return "", fmt.Errorf("derive password hash: %w", err)
	}
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", passwordIterations, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

func passwordMatches(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations < 100_000 || iterations > 2_000_000 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil || len(salt) < passwordSaltBytes {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(expected) != passwordKeyBytes {
		return false
	}
	actual, err := pbkdf2.Key(sha256.New, password, salt, iterations, len(expected))
	return err == nil && subtle.ConstantTimeCompare(actual, expected) == 1
}
func (s *AuthService) Register(ctx context.Context, householdID int64, email, password string, role model.Role) (model.Member, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if householdID <= 0 || len(password) < 10 || domain.ValidateEmail(email) != nil {
		return model.Member{}, model.ErrInvalid
	}
	hash, err := hashPassword(password)
	if err != nil {
		return model.Member{}, err
	}
	return s.Repo.AddMember(ctx, householdID, email, hash, role)
}
func (s *AuthService) Login(ctx context.Context, householdID int64, email, password string) (model.Session, model.Member, error) {
	m, hash, err := s.Repo.FindMember(ctx, householdID, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		return model.Session{}, model.Member{}, model.ErrForbidden
	}
	if !m.Active || !passwordMatches(hash, password) {
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
	actualRank, actualOK := rank[actual]
	requiredRank, requiredOK := rank[required]
	if !actualOK || !requiredOK || actualRank < requiredRank {
		return fmt.Errorf("%w: role %s does not satisfy %s", model.ErrForbidden, actual, required)
	}
	return nil
}
