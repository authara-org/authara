package auth

import (
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

	OAuthID string
}

type LoginInput struct {
	Provider domain.Provider

	Username string
	Email    string
	Password string

	OAuthID         string
	InvitationToken string
}

type OAuthIdentityInput struct {
	Provider domain.Provider

	Username              string
	Email                 string
	ProviderUserID        string
	ProviderEmailVerified bool
}
