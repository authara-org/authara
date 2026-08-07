package token

import "errors"

var (
	ErrInvalidToken         = errors.New("invalid access token")
	ErrExpiredToken         = errors.New("expired access token")
	ErrUnknownKey           = errors.New("unknown signing key")
	ErrInvalidClaims        = errors.New("invalid token claims")
	ErrRevokedToken         = errors.New("revoked access token")
	ErrInvalidRoleNamespace = errors.New("invalid role namespace")
)
