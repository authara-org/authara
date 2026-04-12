package domain

import (
	"time"

	"github.com/google/uuid"
)

type ChallengePurpose string

const (
	ChallengePurposeSignup        ChallengePurpose = "signup"
	ChallengePurposePasswordReset ChallengePurpose = "password_reset"
	ChallengePurposeEmailChange   ChallengePurpose = "email_change"
)

type Challenge struct {
	ID uuid.UUID

	CreatedAt  time.Time
	UpdatedAt  time.Time
	ExpiresAt  time.Time
	ConsumedAt *time.Time

	Purpose      ChallengePurpose
	Email        string
	AttemptCount int
	MaxAttempts  int

	ResendCount int
	MaxResends  int
	LastSentAt  *time.Time
}

type VerificationCode struct {
	ID          uuid.UUID
	ChallengeID uuid.UUID

	CreatedAt time.Time
	ExpiresAt time.Time

	CodeHash string
}

func (c Challenge) IsConsumed() bool {
	return c.ConsumedAt != nil
}

func (c Challenge) IsExpired(now time.Time) bool {
	return now.After(c.ExpiresAt)
}

func (c Challenge) HasAttemptsRemaining() bool {
	return c.AttemptCount < c.MaxAttempts
}
