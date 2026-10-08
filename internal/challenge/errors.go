package challenge

import "errors"

var (
	ErrChallengeExpired            = errors.New("challenge expired")
	ErrChallengeConsumed           = errors.New("challenge already consumed")
	ErrTooManyAttempts             = errors.New("too many verification attempts")
	ErrTooManyResends              = errors.New("too many resend attempts")
	ErrResendTooSoon               = errors.New("resend requested too soon")
	ErrInvalidVerificationCode     = errors.New("invalid verification code")
	ErrUnsupportedChallengePurpose = errors.New("unsupported challenge purpose")
	ErrPasswordResetUnavailable    = errors.New("password reset unavailable for account")
	ErrEmailChangeNotAuthorized    = errors.New("email change not authorized for session")
	ErrEmailAlreadyInUse           = errors.New("email already in use")
	ErrEmailVerificationRequired   = errors.New("email verification required")
	ErrEmailVerificationInvalid    = errors.New("email verification transaction is invalid or expired")
)
