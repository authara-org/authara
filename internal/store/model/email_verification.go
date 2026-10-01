package model

import (
	"time"

	"github.com/google/uuid"
)

type EmailVerificationTransaction struct {
	ID                   uuid.UUID  `db:"id"`
	CreatedAt            time.Time  `db:"created_at"`
	UserID               uuid.UUID  `db:"user_id"`
	OriginalSessionID    uuid.UUID  `db:"original_session_id"`
	Audience             string     `db:"audience"`
	AuthenticationMethod string     `db:"authentication_method"`
	ReturnTo             string     `db:"return_to"`
	ExpiresAt            time.Time  `db:"expires_at"`
	ConsumedAt           *time.Time `db:"consumed_at"`
	ChallengeID          *uuid.UUID `db:"challenge_id"`
	TargetEmail          *string    `db:"target_email"`
}

func (EmailVerificationTransaction) TableName() string {
	return "email_verification_transactions"
}
