package model

import (
	"time"

	"github.com/google/uuid"
)

type RefreshToken struct {
	ID        uuid.UUID `db:"id"`
	CreatedAt time.Time `db:"created_at"`

	SessionID uuid.UUID `db:"session_id"`

	TokenHash string `db:"token_hash"`

	ExpiresAt  time.Time  `db:"expires_at"`
	ConsumedAt *time.Time `db:"consumed_at"`
}

func (RefreshToken) TableName() string {
	return "refresh_tokens"
}
