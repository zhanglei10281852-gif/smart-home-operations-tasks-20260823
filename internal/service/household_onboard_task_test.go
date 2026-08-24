package service

import (
	"context"
	"errors"
	"testing"

	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
)

type onboardingFailureRepository struct{ created bool }

func (r *onboardingFailureRepository) CreateHousehold(context.Context, string, string, int64) (model.Household, error) {
	r.created = true
	return model.Household{ID: 11, Name: "orphaned"}, nil
}
func (r *onboardingFailureRepository) CreateHouseholdWithOwner(context.Context, string, string, int64, string, string) (model.Household, model.Member, error) {
	return model.Household{}, model.Member{}, errors.New("unused")
}
func (r *onboardingFailureRepository) AddMember(context.Context, int64, string, string, model.Role) (model.Member, error) {
	return model.Member{}, errors.New("owner insert failed")
}

func TestOnboardDoesNotLeaveHouseholdWhenOwnerInsertFails(t *testing.T) {
	repository := &onboardingFailureRepository{}
	service := NewHouseholds(repository)
	_, _, err := service.Onboard(context.Background(), "family", "UTC", 100, "owner@example.com", "correct horse battery staple")
	if err == nil || !repository.created {
		t.Fatalf("expected owner failure after household insert, created=%v err=%v", repository.created, err)
	}
	t.Fatal("onboarding returned an error after leaving a persisted household without an owner")
}
