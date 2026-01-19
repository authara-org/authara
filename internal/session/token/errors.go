package token

import "errors"

var (
	ErrInvalidToken  = errors.New("invalid access token")
	ErrExpiredToken  = errors.New("expired access token")
	ErrUnknownKey    = errors.New("unknown signing key")
	ErrInvalidClaims = errors.New("invalid token claims")
)
