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
	ErrCannotRemoveLastAuthMethod      = errors.New("cannot remove last auth method")
	ErrCannotRemoveLastAuthProvider    = ErrCannotRemoveLastAuthMethod
	ErrPasswordAlreadyExists           = errors.New("password provider already exists")
	ErrPendingProviderLinkExpired      = errors.New("pending provider link expired")
	ErrPendingProviderLinkInvalid      = errors.New("pending provider link invalid")
	ErrPendingProviderLinkNeedsProof   = errors.New("pending provider link requires proof")
	ErrProviderEmailNotVerified        = errors.New("provider email is not verified")
	ErrProviderDisabled                = errors.New("provider disabled")
	ErrPasswordProviderMissing         = errors.New("password provider missing")
	ErrPasskeyNotFound                 = errors.New("passkey not found")
	ErrPasskeyAlreadyExists            = errors.New("passkey already exists")
	ErrPasskeyRegistrationInvalid      = errors.New("passkey registration invalid")
	ErrPasskeyAuthenticationInvalid    = errors.New("passkey authentication invalid")
)
