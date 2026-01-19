package domain

import (
	"time"

	"github.com/google/uuid"
)

type Session struct {
	ID     uuid.UUID
	UserID uuid.UUID

	CreatedAt time.Time
	UpdatedAt time.Time

	ExpiresAt time.Time
	RevokedAt *time.Time

	UserAgent string
}
