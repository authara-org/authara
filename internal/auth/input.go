package auth

import "github.com/authara-org/authara/internal/domain"

type SignupInput struct {
	Provider domain.Provider

	Username string
	Email    string
	Password string

	OAuthID string
}

type LoginInput struct {
	Provider domain.Provider

	Username string
	Email    string
	Password string

	OAuthID string
}
