package auth

import "github.com/authara-org/authara/internal/domain"

type SignupInput struct {
	Provider domain.Provider

	Username     string
	Email        string
	PasswordHash string

	OAuthID string
}

type LoginInput struct {
	Provider domain.Provider

	Username string
	Email    string
	Password string

	OAuthID string
}

type OAuthIdentityInput struct {
	Provider domain.Provider

	Username              string
	Email                 string
	ProviderUserID        string
	ProviderEmailVerified bool
}
