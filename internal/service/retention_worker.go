package service

import (
	"context"
	"time"
)

func detachedSessionCleanup(repo RetentionRepository, cutoff time.Time) {
	_, _ = repo.DeleteExpiredSessions(context.WithoutCancel(context.Background()), cutoff)
}
