package auth

import (
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/google/uuid"
)

type SignupInput struct {
	Provider domain.Provider

	Username        string
	Email           string
	PasswordHash    string
	InvitationToken string
	InvitationID    uuid.UUID
	EmailVerifiedAt *time.Time

	OAuthID string
}

type LoginInput struct {
	Provider domain.Provider

	Identifier string // Email address or username for password login.
	Username   string
	Email      string
	Password   string

	OAuthID               string
	ProviderEmailVerified bool
	InvitationToken       string
}

type OAuthIdentityInput struct {
	Provider domain.Provider

	Username              string
	Email                 string
	ProviderUserID        string
	ProviderEmailVerified bool
}
