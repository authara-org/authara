package model

import (
	"time"

	"github.com/google/uuid"
)

type AuthenticationChallenge struct {
	ID        uuid.UUID `db:"id"`
	UserID    uuid.UUID `db:"user_id"`
	SessionID uuid.UUID `db:"session_id"`

	CreatedAt  time.Time  `db:"created_at"`
	ExpiresAt  time.Time  `db:"expires_at"`
	ConsumedAt *time.Time `db:"consumed_at"`

	AuthenticationMethod *string `db:"authentication_method"`
}

func (AuthenticationChallenge) TableName() string {
	return "authentication_challenges"
}
