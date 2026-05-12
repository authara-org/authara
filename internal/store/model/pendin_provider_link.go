package model

import (
	"time"

	"github.com/google/uuid"
)

type PendingProviderLink struct {
	ID uuid.UUID `db:"id"`

	UserID      uuid.UUID  `db:"user_id"`
	SessionID   *uuid.UUID `db:"session_id"`
	ChallengeID *uuid.UUID `db:"challenge_id"`
	Provider    string     `db:"provider"`

	ProviderUserID        *string `db:"provider_user_id"`
	ProviderEmail         *string `db:"provider_email"`
	ProviderEmailVerified bool    `db:"provider_email_verified"`
	Purpose               string  `db:"purpose"`

	ExpiresAt  time.Time  `db:"expires_at"`
	ConsumedAt *time.Time `db:"consumed_at"`

	CreatedAt time.Time `db:"created_at"`
}

func (PendingProviderLink) TableName() string {
	return "pending_provider_links"
}
