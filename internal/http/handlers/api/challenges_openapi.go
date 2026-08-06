package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/challenge"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/httputil"
	"github.com/authara-org/authara/internal/http/kit/response"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/session/token"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (h *APIHandler) StartSignupChallenge(ctx context.Context, request contract.StartSignupChallengeRequestObject) (contract.StartSignupChallengeResponseObject, error) {
	r, ok := contractRequest(ctx)
	if !ok {
		return startSignupChallengeError(responseCodeInternalError(), "API contract error."), nil
	}
	if !h.challengeAvailable() {
		return startSignupChallengeError(responseCodeNotFound(), "Challenge verification is not enabled."), nil
	}
	in, code, message, ok := signupInputFromBody(request.Body)
	if !ok {
		return startSignupChallengeError(code, message), nil
	}
	_, code, message, ok = appAudience(request.Params.Audience)
	if !ok {
		return startSignupChallengeError(code, message), nil
	}
	passwordHash, code, message, ok := h.prepareContractSignup(ctx, r, in)
	if !ok {
		return startSignupChallengeError(code, message), nil
	}
	out, code, message, ok := h.contractStartSignupChallenge(ctx, in.Email, passwordHash, in.InvitationCode)
	if !ok {
		return startSignupChallengeError(code, message), nil
	}
	return contract.StartSignupChallenge202JSONResponse(out), nil
}

func (h *APIHandler) VerifySignupChallenge(ctx context.Context, request contract.VerifySignupChallengeRequestObject) (contract.VerifySignupChallengeResponseObject, error) {
	r, ok := contractRequest(ctx)
	if !ok {
		return verifySignupChallengeError(responseCodeInternalError(), "API contract error."), nil
	}
	if !h.challengeAvailable() {
		return verifySignupChallengeError(responseCodeNotFound(), "Challenge verification is not enabled."), nil
	}
	if request.Body == nil || !isSixDigitCode(strings.TrimSpace(request.Body.Code)) {
		return verifySignupChallengeError(responseCodeInvalidRequest(), "Invalid challenge request."), nil
	}
	_, code, message, ok := appAudience(request.Params.Audience)
	if !ok {
		return verifySignupChallengeError(code, message), nil
	}
	if h.Limiter != nil {
		allowed, err := h.Limiter.AllowChallengeVerifyAttempt(ctx, httputil.ClientIP(r))
		if err != nil || !allowed {
			return verifySignupChallengeError(responseCodeRateLimited(), "Too many verification attempts. Please try again later."), nil
		}
	}

	var user domain.User
	var accessToken string
	var refreshToken string
	var signupFailed bool
	var sessionFailed bool
	now := time.Now().UTC()
	_, err := h.Challenge.VerifySignupChallenge(ctx, request.Body.ChallengeId, strings.TrimSpace(request.Body.Code), h.Verification, now, func(txCtx context.Context, action domain.PendingSignupAction) error {
		signup := auth.SignupInput{
			Provider:     domain.ProviderPassword,
			Username:     action.Username,
			Email:        action.Email,
			PasswordHash: action.PasswordHash,
		}
		if action.InvitationID != nil {
			signup.InvitationID = *action.InvitationID
		}
		var err error
		user, err = h.Auth.Signup(txCtx, signup)
		if err != nil {
			signupFailed = true
			return err
		}
		accessToken, refreshToken, err = h.Session.CreateSession(txCtx, user.ID, token.AudienceApp, r.UserAgent(), now)
		if err != nil {
			sessionFailed = true
		}
		return err
	})
	if err != nil {
		if isExpectedChallengeVerifyError(err) {
			return verifySignupChallengeError(responseCodeInvalidRequest(), "Invalid or expired verification code."), nil
		}
		if signupFailed {
			code := authSignupErrorCode(err)
			message := "Could not create account. Please check your details."
			if code == responseCodeInternalError() {
				message = "Signup error."
			}
			return verifySignupChallengeError(code, message), nil
		}
		if sessionFailed {
			code := sessionErrorCode(err)
			message := "Session error."
			if code == responseCodeForbidden() {
				message = "Account cannot access requested audience."
			}
			return verifySignupChallengeError(code, message), nil
		}
		return verifySignupChallengeError(responseCodeInternalError(), "Challenge error."), nil
	}
	header := make(http.Header)
	session.SetAccessToken(contract.HeaderWriter(header), accessToken, int(h.AccessTTL.Seconds()))
	session.SetRefreshToken(contract.HeaderWriter(header), refreshToken, int(h.RefreshTTL.Seconds()))
	return contract.VerifySignupChallenge201HeadersResponse{
		Header: header,
		Body:   toContractAuthSession(user, accessToken, refreshToken),
	}, nil
}

func (h *APIHandler) ResendChallenge(ctx context.Context, request contract.ResendChallengeRequestObject) (contract.ResendChallengeResponseObject, error) {
	r, ok := contractRequest(ctx)
	if !ok {
		return resendChallengeError(responseCodeInternalError(), "API contract error."), nil
	}
	if h.Challenge == nil {
		return resendChallengeError(responseCodeInternalError(), "Challenge error."), nil
	}
	if request.Body == nil {
		return resendChallengeError(responseCodeInvalidRequest(), "Invalid challenge request."), nil
	}
	if h.Limiter != nil {
		allowed, err := h.Limiter.AllowChallengeResendAttempt(ctx, httputil.ClientIP(r))
		if err != nil || !allowed {
			return resendChallengeError(responseCodeRateLimited(), "Too many resend attempts. Please try again later."), nil
		}
	}
	err := h.Challenge.ResendChallenge(ctx, request.Body.ChallengeId, time.Now().UTC())
	if err != nil && !isOpaqueChallengeResendError(err) {
		return resendChallengeError(responseCodeInternalError(), "Challenge error."), nil
	}
	return contract.ResendChallenge204Response{}, nil
}

func (h *APIHandler) challengeAvailable() bool {
	return h.ChallengeEnabled && h.Challenge != nil && h.Verification != nil
}

func (h *APIHandler) contractStartSignupChallenge(
	ctx context.Context,
	email string,
	passwordHash string,
	invitationCode string,
) (contract.SignupChallenge, response.ErrorCode, string, bool) {
	if h.Challenge == nil {
		return contract.SignupChallenge{}, response.CodeInternalError, "Challenge error.", false
	}
	exists, err := h.Auth.UserExistsByEmail(ctx, email)
	if err != nil {
		return contract.SignupChallenge{}, response.CodeInternalError, "Challenge error.", false
	}
	now := time.Now().UTC()
	var challengeID openapi_types.UUID
	if exists {
		challengeID, err = h.Challenge.CreateOpaqueChallenge(ctx, now, domain.ChallengePurposeSignup, email)
	} else {
		invitationID, err := h.invitationIDForSignupCode(ctx, email, invitationCode)
		if err != nil {
			code := authSignupErrorCode(err)
			return contract.SignupChallenge{}, code, authSignupErrorMessage(err, code), false
		}
		challengeID, err = h.Challenge.CreateSignupChallenge(ctx, challenge.CreateSignupChallengeInput{
			Email:        email,
			PasswordHash: passwordHash,
			InvitationID: invitationID,
		}, now)
	}
	if err != nil {
		return contract.SignupChallenge{}, response.CodeInternalError, "Challenge error.", false
	}
	return contract.SignupChallenge{ChallengeId: challengeID}, "", "", true
}
