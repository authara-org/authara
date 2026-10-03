package api

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/challenge"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/httputil"
	"github.com/authara-org/authara/internal/http/kit/validation"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/identity"
	"github.com/authara-org/authara/internal/store"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (h *APIHandler) StartPasswordResetChallenge(ctx context.Context, request contract.StartPasswordResetChallengeRequestObject) (contract.StartPasswordResetChallengeResponseObject, error) {
	r, ok := contractRequest(ctx)
	if !ok {
		return startPasswordResetChallengeError(responseCodeInternalError(), "API contract error."), nil
	}
	if h.Challenge == nil {
		return startPasswordResetChallengeError(responseCodeInternalError(), "Challenge error."), nil
	}
	if request.Body == nil {
		return startPasswordResetChallengeError(responseCodeInvalidRequest(), "Invalid JSON body."), nil
	}

	email := identity.CanonicalEmail(string(request.Body.Email))
	password := request.Body.NewPassword
	if !validation.IsValidEmail(email) {
		return startPasswordResetChallengeError(responseCodeInvalidRequest(), "Please provide a valid email address."), nil
	}
	if err := h.Auth.ValidatePassword(ctx, password); err != nil {
		code, message := h.passwordPolicyError(err)
		return startPasswordResetChallengeError(code, message), nil
	}
	if h.Limiter != nil {
		allowed, err := h.Limiter.AllowPasswordResetAttempt(ctx, httputil.ClientIP(r), email)
		if code, message, ok := h.rateLimitResult(allowed, err, "Too many reset attempts. Please try again later."); !ok {
			return startPasswordResetChallengeError(code, message), nil
		}
	}

	passwordHash, err := h.Auth.HashPassword(ctx, password)
	if err != nil {
		code, message := h.passwordPolicyError(err)
		return startPasswordResetChallengeError(code, message), nil
	}

	now := time.Now().UTC()
	var challengeID openapi_types.UUID
	user, err := h.Auth.GetUserByEmail(ctx, email)
	switch {
	case err == nil:
		challengeID, err = h.Challenge.CreatePasswordResetChallenge(ctx, challenge.CreatePasswordResetChallengeInput{
			UserID:       user.ID,
			Email:        user.Email,
			PasswordHash: passwordHash,
		}, now)
	case errors.Is(err, store.ErrUserNotFound):
		challengeID, err = h.Challenge.CreateOpaqueChallenge(ctx, now, domain.ChallengePurposePasswordReset, email)
	default:
		return startPasswordResetChallengeError(responseCodeInternalError(), "Password reset error."), nil
	}
	if err != nil {
		return startPasswordResetChallengeError(responseCodeInternalError(), "Password reset error."), nil
	}

	return contract.StartPasswordResetChallenge202JSONResponse{ChallengeId: challengeID}, nil
}

func (h *APIHandler) VerifyPasswordResetChallenge(ctx context.Context, request contract.VerifyPasswordResetChallengeRequestObject) (contract.VerifyPasswordResetChallengeResponseObject, error) {
	r, ok := contractRequest(ctx)
	if !ok {
		return verifyPasswordResetChallengeError(responseCodeInternalError(), "API contract error."), nil
	}
	if h.Challenge == nil || h.Verification == nil {
		return verifyPasswordResetChallengeError(responseCodeInternalError(), "Challenge error."), nil
	}
	if request.Body == nil || !isSixDigitCode(strings.TrimSpace(request.Body.Code)) {
		return verifyPasswordResetChallengeError(responseCodeInvalidRequest(), "Invalid challenge request."), nil
	}
	if h.Limiter != nil {
		allowed, err := h.Limiter.AllowChallengeVerifyAttempt(ctx, httputil.ClientIP(r))
		if code, message, ok := h.rateLimitResult(allowed, err, "Too many verification attempts. Please try again later."); !ok {
			return verifyPasswordResetChallengeError(code, message), nil
		}
	}

	err := h.Challenge.CompletePasswordResetChallenge(
		ctx,
		request.Body.ChallengeId,
		strings.TrimSpace(request.Body.Code),
		h.Verification,
		time.Now().UTC(),
	)
	if err != nil {
		if errors.Is(err, challenge.ErrPasswordResetUnavailable) {
			return verifyPasswordResetChallengeError(responseCodeInvalidRequest(), "Password reset is no longer available for this request. Start again or sign in with a configured provider or passkey."), nil
		}
		if isExpectedPasswordResetVerifyError(err) {
			return verifyPasswordResetChallengeError(responseCodeInvalidRequest(), "Invalid or expired verification code."), nil
		}
		return verifyPasswordResetChallengeError(responseCodeInternalError(), "Challenge error."), nil
	}

	return contract.VerifyPasswordResetChallenge204Response{}, nil
}
