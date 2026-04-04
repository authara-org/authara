package store

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrUserNotFound                  = errors.New("user not found")
	ErrSessionNotFound               = errors.New("session not found")
	ErrorAuthProviderNotFound        = errors.New("auth_provider not found")
	ErrRefreshTokenNotFound          = errors.New("refresh_token not found")
	ErrorChallengeNotFound           = errors.New("challenge not found")
	ErrorVerificationCodeNotFound    = errors.New("verification code not found")
	ErrorEmailJobNotFound            = errors.New("email job not found")
	ErrorPendingSignupActionNotFound = errors.New("pending signup action not found")
)

const (
	ConstraintUserEmail    = "unique_user_email"
	ConstraintUserUsername = "unique_user_username"

	uniqueViolationCode = "23505"
)

func IsUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}

	if pgErr.Code != uniqueViolationCode {
		return false
	}

	if constraint == "" {
		return true
	}

	return pgErr.ConstraintName == constraint
}
