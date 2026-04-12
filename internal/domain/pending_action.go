package domain

import (
	"time"

	"github.com/google/uuid"
)

type PendingSignupAction struct {
	ID          uuid.UUID
	ChallengeID uuid.UUID

	CreatedAt time.Time
	UpdatedAt time.Time

	Email        string
	Username     string
	PasswordHash string
}

type PendingPasswordReset struct {
	ID           uuid.UUID
	CreatedAt    time.Time
	ChallengeID  uuid.UUID
	UserID       uuid.UUID
	PasswordHash string
}

type PendingEmailChange struct {
	ID          uuid.UUID
	CreatedAt   time.Time
	ChallengeID uuid.UUID
	UserID      uuid.UUID
	OldEmail    string
	NewEmail    string
}
