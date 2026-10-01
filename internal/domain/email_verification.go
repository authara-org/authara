package domain

import (
	"time"

	"github.com/google/uuid"
)

type EmailVerificationTransaction struct {
	ID                   uuid.UUID
	CreatedAt            time.Time
	UserID               uuid.UUID
	OriginalSessionID    uuid.UUID
	Audience             string
	AuthenticationMethod AuthenticationMethod
	ReturnTo             string
	ExpiresAt            time.Time
	ConsumedAt           *time.Time
	ChallengeID          *uuid.UUID
	TargetEmail          *string
}

func (t EmailVerificationTransaction) IsActive(now time.Time) bool {
	return t.ConsumedAt == nil && t.ExpiresAt.After(now)
}
