package auth

import "errors"

var (
	ErrInvalidCredentials    = errors.New("invalid credentials")
	ErrUserAlreadyExists     = errors.New("user already exists")
	ErrUnsupportedProvider   = errors.New("auth provider is not supported")
	ErrAccountExistsMustLink = errors.New("account exists; must link provider explicitly")
)
