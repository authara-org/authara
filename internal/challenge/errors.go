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
)
