package domain

import (
	"time"

	"github.com/google/uuid"
)

type Session struct {
	ID                   uuid.UUID
	UserID               uuid.UUID
	ActiveOrganizationID uuid.UUID

	CreatedAt time.Time
	UpdatedAt time.Time

	ExpiresAt time.Time
	RevokedAt *time.Time

	AuthenticatedAt      *time.Time
	AuthenticationMethod AuthenticationMethod

	UserAgent string
}

type AuthenticationMethod string

const (
	AuthenticationMethodPassword AuthenticationMethod = "password"
	AuthenticationMethodPasskey  AuthenticationMethod = "passkey"
	AuthenticationMethodGoogle   AuthenticationMethod = "google"
	AuthenticationMethodApple    AuthenticationMethod = "apple"
)

type AuthenticationChallenge struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	SessionID uuid.UUID

	CreatedAt  time.Time
	ExpiresAt  time.Time
	ConsumedAt *time.Time

	AuthenticationMethod AuthenticationMethod
}

func (c AuthenticationChallenge) IsConsumed() bool {
	return c.ConsumedAt != nil
}

func (c AuthenticationChallenge) IsExpired(now time.Time) bool {
	return !now.Before(c.ExpiresAt)
}
