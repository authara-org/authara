package auth

import "errors"

var (
	ErrInvalidCredentials    = errors.New("invalid credentials")
	ErrUserAlreadyExists     = errors.New("user already exists")
	ErrUnsupportedProvider   = errors.New("auth provider is not supported")
	ErrAccountExistsMustLink = errors.New("account exists; must link provider explicitly")
	ErrUsernameTaken         = errors.New("username already taken")
	ErrInvalidUsername       = errors.New("invalid username")
	ErrNoRolesInContext      = errors.New("no roles in context for user")
	ErrEmailNotAllowed       = errors.New("email is not allowed to access the application")
)
