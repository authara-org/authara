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
	r, out, ok := contractRequest(ctx)
	if !ok {
		return out, nil
	}
	if !h.challengeAvailable() {
		return routeError(StartSignupChallengeErrors, responseCodeNotFound(), "Challenge verification is not enabled."), nil
	}
	in, out, ok := signupInputFromBody(request.Body, StartSignupChallengeErrors)
	if !ok {
		return out, nil
	}
	_, out, ok = appAudience(request.Params.Audience, StartSignupChallengeErrors)
	if !ok {
		return out, nil
	}
	passwordHash, out, ok := h.prepareContractSignup(ctx, r, in, StartSignupChallengeErrors)
	if !ok {
		return out, nil
	}
	return h.contractStartSignupChallenge(ctx, in.Email, passwordHash, in.InvitationCode, StartSignupChallengeErrors), nil
}

func (h *APIHandler) VerifySignupChallenge(ctx context.Context, request contract.VerifySignupChallengeRequestObject) (contract.VerifySignupChallengeResponseObject, error) {
	r, out, ok := contractRequest(ctx)
	if !ok {
		return out, nil
	}
	if !h.challengeAvailable() {
		return routeError(VerifySignupChallengeErrors, responseCodeNotFound(), "Challenge verification is not enabled."), nil
	}
	if request.Body == nil || !isSixDigitCode(strings.TrimSpace(request.Body.Code)) {
		return invalidChallengeResponse(VerifySignupChallengeErrors), nil
	}
	_, out, ok = appAudience(request.Params.Audience, VerifySignupChallengeErrors)
	if !ok {
		return out, nil
	}
	if h.Limiter != nil {
		allowed, err := h.Limiter.AllowChallengeVerifyAttempt(ctx, httputil.ClientIP(r))
		if err != nil || !allowed {
			return routeError(VerifySignupChallengeErrors, responseCodeRateLimited(), "Too many verification attempts. Please try again later."), nil
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
			return routeError(VerifySignupChallengeErrors, responseCodeInvalidRequest(), "Invalid or expired verification code."), nil
		}
		if signupFailed {
			code := authSignupErrorCode(err)
			message := "Could not create account. Please check your details."
			if code == responseCodeInternalError() {
				message = "Signup error."
			}
			return routeError(VerifySignupChallengeErrors, code, message), nil
		}
		if sessionFailed {
			code := sessionErrorCode(err)
			message := "Session error."
			if code == responseCodeForbidden() {
				message = "Account cannot access requested audience."
			}
			return routeError(VerifySignupChallengeErrors, code, message), nil
		}
		return routeError(VerifySignupChallengeErrors, responseCodeInternalError(), "Challenge error."), nil
	}
	header := make(http.Header)
	session.SetAccessToken(contract.HeaderWriter(header), accessToken, int(h.AccessTTL.Seconds()))
	session.SetRefreshToken(contract.HeaderWriter(header), refreshToken, int(h.RefreshTTL.Seconds()))
	return contract.JSON(http.StatusCreated, toContractAuthSession(user, accessToken, refreshToken), header), nil
}

func (h *APIHandler) ResendChallenge(ctx context.Context, request contract.ResendChallengeRequestObject) (contract.ResendChallengeResponseObject, error) {
	r, out, ok := contractRequest(ctx)
	if !ok {
		return out, nil
	}
	if !h.challengeAvailable() {
		return routeError(ResendChallengeErrors, responseCodeNotFound(), "Challenge verification is not enabled."), nil
	}
	if request.Body == nil {
		return invalidChallengeResponse(ResendChallengeErrors), nil
	}
	if h.Limiter != nil {
		allowed, err := h.Limiter.AllowChallengeResendAttempt(ctx, httputil.ClientIP(r))
		if err != nil || !allowed {
			return routeError(ResendChallengeErrors, responseCodeRateLimited(), "Too many resend attempts. Please try again later."), nil
		}
	}
	err := h.Challenge.ResendChallenge(ctx, request.Body.ChallengeId, time.Now().UTC())
	if err != nil && !isOpaqueChallengeResendError(err) {
		return routeError(ResendChallengeErrors, responseCodeInternalError(), "Challenge error."), nil
	}
	return contract.NoContent(), nil
}

func (h *APIHandler) challengeAvailable() bool {
	return h.ChallengeEnabled && h.Challenge != nil && h.Verification != nil
}

func (h *APIHandler) contractStartSignupChallenge(
	ctx context.Context,
	email string,
	passwordHash string,
	invitationCode string,
	routeErrors map[response.ErrorCode]response.ErrorSpec,
) contract.Response {
	if h.Challenge == nil {
		return routeError(routeErrors, response.CodeInternalError, "Challenge error.")
	}
	exists, err := h.Auth.UserExistsByEmail(ctx, email)
	if err != nil {
		return routeError(routeErrors, response.CodeInternalError, "Challenge error.")
	}
	now := time.Now().UTC()
	var challengeID openapi_types.UUID
	if exists {
		challengeID, err = h.Challenge.CreateOpaqueChallenge(ctx, now, domain.ChallengePurposeSignup, email)
	} else {
		invitationID, err := h.invitationIDForSignupCode(ctx, email, invitationCode)
		if err != nil {
			code := authSignupErrorCode(err)
			return routeError(routeErrors, code, authSignupErrorMessage(err, code))
		}
		challengeID, err = h.Challenge.CreateSignupChallenge(ctx, challenge.CreateSignupChallengeInput{
			Email:        email,
			PasswordHash: passwordHash,
			InvitationID: invitationID,
		}, now)
	}
	if err != nil {
		return routeError(routeErrors, response.CodeInternalError, "Challenge error.")
	}
	return contract.JSON(http.StatusAccepted, contract.SignupChallenge{ChallengeId: challengeID})
}

func invalidChallengeResponse(routeErrors map[response.ErrorCode]response.ErrorSpec) contract.Response {
	return routeError(routeErrors, response.CodeInvalidRequest, "Invalid challenge request.")
}
