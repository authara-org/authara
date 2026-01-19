package store

import "errors"

var (
	ErrUserNotFound           = errors.New("user not found")
	ErrSessionNotFound        = errors.New("session not found")
	ErrorAuthProviderNotFound = errors.New("auth_provider not found")
	ErrRefreshTokenNotFound   = errors.New("refresh_token not found")
)
