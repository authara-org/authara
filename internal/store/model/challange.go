package model

import (
	"time"

	"github.com/google/uuid"
)

type Challenge struct {
	ID        uuid.UUID `db:"id"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`

	Purpose string `db:"purpose"`
	Email   string `db:"email"`

	ExpiresAt    time.Time  `db:"expires_at"`
	ConsumedAt   *time.Time `db:"consumed_at"`
	AttemptCount int        `db:"attempt_count"`
	MaxAttempts  int        `db:"max_attempts"`

	ResendCount int        `db:"resend_count"`
	MaxResends  int        `db:"max_resends"`
	LastSentAt  *time.Time `db:"last_sent_at"`
}

func (Challenge) TableName() string {
	return "challenges"
}

type VerificationCode struct {
	ID        uuid.UUID `db:"id"`
	CreatedAt time.Time `db:"created_at"`

	ChallengeID uuid.UUID `db:"challenge_id"`
	CodeHash    string    `db:"code_hash"`
	ExpiresAt   time.Time `db:"expires_at"`
}

func (VerificationCode) TableName() string {
	return "verification_codes"
}
