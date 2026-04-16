package domain

import (
	"time"

	"github.com/google/uuid"
)

type PendingProviderLink struct {
	ID        uuid.UUID
	CreatedAt time.Time

	UserID    uuid.UUID
	SessionID uuid.UUID
	Provider  Provider

	ExpiresAt  time.Time
	ConsumedAt *time.Time
}
