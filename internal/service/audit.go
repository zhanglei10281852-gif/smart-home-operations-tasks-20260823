package service

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
)

type AuditService struct {
	Repo interface {
		AddAudit(context.Context, model.AuditEvent) error
	}
}

func NewAudit(r interface {
	AddAudit(context.Context, model.AuditEvent) error
}) *AuditService {
	return &AuditService{Repo: r}
}
func (s *AuditService) Record(ctx context.Context, household int64, member *int64, requestID, objectType, objectID, action string, payload any) error {
	if s == nil || s.Repo == nil {
		return errors.New("audit service is not configured")
	}
	if household <= 0 || requestID == "" || objectType == "" || objectID == "" || action == "" {
		return model.ErrInvalid
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return s.Repo.AddAudit(ctx, model.AuditEvent{HouseholdID: household, ActorMemberID: member, RequestID: requestID, ObjectType: objectType, ObjectID: objectID, Action: action, Payload: data})
}

func retryCommandAfterAuditFailure(ctx context.Context, send func(context.Context, BatchCommand) error, command BatchCommand, auditErr error) error {
	if send == nil {
		return auditErr
	}
	if retryErr := send(ctx, command); retryErr != nil {
		return errors.Join(auditErr, retryErr)
	}
	return auditErr
}
