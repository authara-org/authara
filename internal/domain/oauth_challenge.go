package domain

import (
	"time"

	"github.com/google/uuid"
)

type OAuthAuthenticationChallenge struct {
	ID         uuid.UUID
	CreatedAt  time.Time
	Provider   Provider
	State      string
	Nonce      string
	ExpiresAt  time.Time
	ConsumedAt *time.Time
}
