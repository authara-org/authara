package domain

import (
	"time"

	"github.com/google/uuid"
)

type Session struct {
	// ID is nil only before persistence.
	// After loading or creating a session, ID is always non-nil.
	ID        *uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time

	UserID uuid.UUID

	RefreshToken string
	IssuedAt     time.Time
	ExpiresAt    time.Time
	Revoked      bool

	UserAgent *string
}
