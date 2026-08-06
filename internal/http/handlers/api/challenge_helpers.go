package api

import (
	"errors"

	"github.com/authara-org/authara/internal/challenge"
	"github.com/authara-org/authara/internal/store"
)

func isSixDigitCode(code string) bool {
	if len(code) != 6 {
		return false
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isExpectedChallengeVerifyError(err error) bool {
	return errors.Is(err, challenge.ErrChallengeExpired) ||
		errors.Is(err, challenge.ErrChallengeConsumed) ||
		errors.Is(err, challenge.ErrTooManyAttempts) ||
		errors.Is(err, challenge.ErrInvalidVerificationCode) ||
		errors.Is(err, challenge.ErrUnsupportedChallengePurpose) ||
		errors.Is(err, store.ErrorChallengeNotFound) ||
		errors.Is(err, store.ErrorVerificationCodeNotFound) ||
		errors.Is(err, store.ErrorPendingSignupActionNotFound)
}

func isExpectedPasswordResetVerifyError(err error) bool {
	return isExpectedChallengeVerifyError(err) ||
		errors.Is(err, store.ErrorPendingPasswordResetNotFound)
}

func isOpaqueChallengeResendError(err error) bool {
	return errors.Is(err, challenge.ErrChallengeExpired) ||
		errors.Is(err, challenge.ErrChallengeConsumed) ||
		errors.Is(err, challenge.ErrTooManyResends) ||
		errors.Is(err, challenge.ErrResendTooSoon) ||
		errors.Is(err, store.ErrorChallengeNotFound)
}
