package auth

import "errors"

var (
	ErrInvalidCredentials              = errors.New("invalid credentials")
	ErrUserAlreadyExists               = errors.New("user already exists")
	ErrUnsupportedProvider             = errors.New("auth provider is not supported")
	ErrAccountExistsMustLink           = errors.New("account exists; must link provider explicitly")
	ErrUsernameTaken                   = errors.New("username already taken")
	ErrInvalidUsername                 = errors.New("invalid username")
	ErrNoRolesInContext                = errors.New("no roles in context for user")
	ErrEmailNotAllowed                 = errors.New("email is not allowed to access the application")
	ErrAuthProviderAlreadyLinked       = errors.New("auth provider already linked to another user")
	ErrAuthProviderAlreadyLinkedToUser = errors.New("auth provider already linked to user")
	ErrCannotRemoveLastAuthProvider    = errors.New("cannot remove last auth provider")
	ErrPasswordAlreadyExists           = errors.New("password provider already exists")
	ErrPendingProviderLinkExpired      = errors.New("pending provider link expired")
	ErrPendingProviderLinkInvalid      = errors.New("pending provider link invalid")
)
