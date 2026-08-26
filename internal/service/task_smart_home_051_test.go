package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
)

type blockingSessionRepository struct {
	member    model.Member
	hash      string
	started   chan struct{}
	release   chan struct{}
	persisted chan struct{}
	aborted   chan struct{}
	once      sync.Once
}

type task051Clock struct{ now time.Time }

func (c task051Clock) Now() time.Time { return c.now }

func (r *blockingSessionRepository) AddMember(context.Context, int64, string, string, model.Role) (model.Member, error) {
	return model.Member{}, errors.New("not used")
}
func (r *blockingSessionRepository) FindMember(context.Context, int64, string) (model.Member, string, error) {
	return r.member, r.hash, nil
}
func (r *blockingSessionRepository) CreateSession(ctx context.Context, _ model.Session) error {
	r.once.Do(func() { close(r.started) })
	select {
	case <-r.release:
		close(r.persisted)
		return nil
	case <-ctx.Done():
		close(r.aborted)
		return ctx.Err()
	}
}
func (r *blockingSessionRepository) GetSession(context.Context, string) (model.Session, error) {
	return model.Session{}, errors.New("not used")
}
func (r *blockingSessionRepository) MemberByID(context.Context, int64) (model.Member, error) {
	return model.Member{}, errors.New("not used")
}
func (r *blockingSessionRepository) RevokeSession(context.Context, string, time.Time) error {
	return errors.New("not used")
}

func TestCanceledLoginCannotLeaveOrphanSession(t *testing.T) {
	hash, err := hashPassword("correct-horse-battery")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	repository := &blockingSessionRepository{
		member:    model.Member{ID: 42, HouseholdID: 7, Email: "resident@example.test", Role: model.RoleOperator, Active: true},
		hash:      hash,
		started:   make(chan struct{}),
		release:   make(chan struct{}),
		persisted: make(chan struct{}),
		aborted:   make(chan struct{}),
	}
	clock := task051Clock{now: time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)}
	service := NewAuth(repository, clock)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, _, loginErr := service.Login(ctx, 7, "resident@example.test", "correct-horse-battery")
		result <- loginErr
	}()

	<-repository.started
	cancel()
	if loginErr := <-result; !errors.Is(loginErr, context.Canceled) {
		t.Fatalf("canceled login returned %v", loginErr)
	}
	close(repository.release)
	select {
	case <-repository.persisted:
		t.Fatal("canceled login persisted an orphan session")
	case <-repository.aborted:
	}
}
