package auth

import "github.com/alexlup06/authgate/internal/domain"

type SignupInput struct {
	Provider domain.Provider

	Email    string
	Password string

	OAuthID string
}

type LoginInput struct {
	Provider domain.Provider

	Email    string
	Password string

	OAuthID string
}
