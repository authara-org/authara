package session

import "errors"

var (
	ErrUnauthenticated     = errors.New("unauthenticated")
	ErrSessionExpired      = errors.New("session expired")
	ErrSessionRevoked      = errors.New("session revoked")
	ErrInvalidSession      = errors.New("invalid session")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
	ErrRefreshTokenReuse   = errors.New("refresh token reuse")
	ErrForbidden           = errors.New("forbidden")
)
