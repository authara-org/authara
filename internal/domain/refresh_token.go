package domain

import (
	"time"

	"github.com/google/uuid"
)

type RefreshToken struct {
	ID        uuid.UUID
	CreatedAt time.Time

	SessionID      uuid.UUID
	OrganizationID uuid.UUID
	TokenHash      string

	ExpiresAt  time.Time
	ConsumedAt *time.Time
}
