package passkey

import "github.com/authara-org/authara/internal/auth"

var (
	ErrPasskeyNotFound              = auth.ErrPasskeyNotFound
	ErrPasskeyAlreadyExists         = auth.ErrPasskeyAlreadyExists
	ErrPasskeyRegistrationInvalid   = auth.ErrPasskeyRegistrationInvalid
	ErrPasskeyAuthenticationInvalid = auth.ErrPasskeyAuthenticationInvalid
	ErrCannotRemoveLastAuthMethod   = auth.ErrCannotRemoveLastAuthMethod
	ErrCannotRemoveLastAuthProvider = ErrCannotRemoveLastAuthMethod
)
