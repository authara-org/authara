package domain

import (
	"time"

	"github.com/google/uuid"
)

type Provider string

const (
	ProviderPassword Provider = "password"
	ProviderGoogle   Provider = "google"
)

type AuthProvider struct {
	// ID is nil only before persistence.
	// After loading or creating a authProvider, ID is always non-nil.
	ID        *uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time

	UserID   uuid.UUID
	Provider Provider

	ProviderUserID *string
	PasswordHash   *string
}
