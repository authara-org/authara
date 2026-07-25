package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/challenge"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/httputil"
	"github.com/authara-org/authara/internal/http/kit/response"
	"github.com/authara-org/authara/internal/session/token"
	"github.com/authara-org/authara/internal/store"
	"github.com/google/uuid"
)

const maxChallengeBodyBytes = 4096

type challengeStartedResponse struct {
	ChallengeID string `json:"challenge_id"`
}

type signupVerifyRequest struct {
	ChallengeID string `json:"challenge_id"`
	Code        string `json:"code"`
}

type challengeResendRequest struct {
	ChallengeID string `json:"challenge_id"`
}

func (h *APIHandler) startSignupChallenge(
	w http.ResponseWriter,
	r *http.Request,
	email string,
	passwordHash string,
	invitationCode string,
) {
	if h.Challenge == nil {
		response.WriteError(
			w,
			mustRouteError(SignupPostErrors, response.CodeInternalError),
			"Challenge error.",
		)
		return
	}

	exists, err := h.Auth.UserExistsByEmail(r.Context(), email)
	if err != nil {
		response.WriteError(
			w,
			mustRouteError(SignupPostErrors, response.CodeInternalError),
			"Challenge error.",
		)
		return
	}

	now := time.Now().UTC()
	var challengeID uuid.UUID
	if exists {
		challengeID, err = h.Challenge.CreateOpaqueChallenge(
			r.Context(),
			now,
			domain.ChallengePurposeSignup,
			email,
		)
	} else {
		invitationID, err := h.invitationIDForSignupCode(r.Context(), email, invitationCode)
		if err != nil {
			response.WriteError(
				w,
				mustRouteError(SignupPostErrors, authSignupErrorCode(err)),
				authSignupErrorMessage(err, authSignupErrorCode(err)),
			)
			return
		}
		challengeID, err = h.Challenge.CreateSignupChallenge(
			r.Context(),
			challenge.CreateSignupChallengeInput{
				Email:        email,
				PasswordHash: passwordHash,
				InvitationID: invitationID,
			},
			now,
		)
	}
	if err != nil {
		response.WriteError(
			w,
			mustRouteError(SignupPostErrors, response.CodeInternalError),
			"Challenge error.",
		)
		return
	}

	response.JSON(w, http.StatusAccepted, challengeStartedResponse{ChallengeID: challengeID.String()})
}

func (h *APIHandler) SignupVerifyPost(w http.ResponseWriter, r *http.Request) {
	if !h.requireChallengeAPI(w, SignupVerifyPostErrors) {
		return
	}

	var in signupVerifyRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxChallengeBodyBytes)).Decode(&in); err != nil {
		writeInvalidChallengeRequest(w, SignupVerifyPostErrors)
		return
	}

	in.ChallengeID = strings.TrimSpace(in.ChallengeID)
	in.Code = strings.TrimSpace(in.Code)
	challengeID, err := uuid.Parse(in.ChallengeID)
	if err != nil || !isSixDigitCode(in.Code) {
		writeInvalidChallengeRequest(w, SignupVerifyPostErrors)
		return
	}

	audience, ok := readAudience(w, r, SignupVerifyPostErrors)
	if !ok {
		return
	}
	if audience != token.AudienceApp {
		response.WriteError(
			w,
			mustRouteError(SignupVerifyPostErrors, response.CodeForbidden),
			"Signup only supports app audience.",
		)
		return
	}

	if h.Limiter != nil {
		allowed, err := h.Limiter.AllowChallengeVerifyAttempt(r.Context(), httputil.ClientIP(r))
		if err != nil || !allowed {
			response.WriteError(
				w,
				mustRouteError(SignupVerifyPostErrors, response.CodeRateLimited),
				"Too many verification attempts. Please try again later.",
			)
			return
		}
	}

	var user domain.User
	var accessToken string
	var refreshToken string
	var signupFailed bool
	var sessionFailed bool
	now := time.Now().UTC()
	_, err = h.Challenge.VerifySignupChallenge(
		r.Context(),
		challengeID,
		in.Code,
		h.Verification,
		now,
		func(txCtx context.Context, action domain.PendingSignupAction) error {
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
			accessToken, refreshToken, err = h.Session.CreateSession(
				txCtx,
				user.ID,
				token.AudienceApp,
				r.UserAgent(),
				now,
			)
			if err != nil {
				sessionFailed = true
			}
			return err
		},
	)
	if err != nil {
		if isExpectedChallengeVerifyError(err) {
			response.WriteError(
				w,
				mustRouteError(SignupVerifyPostErrors, response.CodeInvalidRequest),
				"Invalid or expired verification code.",
			)
			return
		}
		if signupFailed {
			code := authSignupErrorCode(err)
			message := "Could not create account. Please check your details."
			if code == response.CodeInternalError {
				message = "Signup error."
			}
			response.WriteError(w, mustRouteError(SignupVerifyPostErrors, code), message)
			return
		}
		if sessionFailed {
			code := sessionErrorCode(err)
			message := "Session error."
			if code == response.CodeForbidden {
				message = "Account cannot access requested audience."
			}
			response.WriteError(w, mustRouteError(SignupVerifyPostErrors, code), message)
			return
		}
		response.WriteError(
			w,
			mustRouteError(SignupVerifyPostErrors, response.CodeInternalError),
			"Challenge error.",
		)
		return
	}

	h.writeSessionResponse(w, user, http.StatusCreated, accessToken, refreshToken)
}

func (h *APIHandler) ChallengeResendPost(w http.ResponseWriter, r *http.Request) {
	if !h.requireChallengeAPI(w, ChallengeResendPostErrors) {
		return
	}

	var in challengeResendRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxChallengeBodyBytes)).Decode(&in); err != nil {
		writeInvalidChallengeRequest(w, ChallengeResendPostErrors)
		return
	}

	challengeID, err := uuid.Parse(strings.TrimSpace(in.ChallengeID))
	if err != nil {
		writeInvalidChallengeRequest(w, ChallengeResendPostErrors)
		return
	}

	if h.Limiter != nil {
		allowed, err := h.Limiter.AllowChallengeResendAttempt(r.Context(), httputil.ClientIP(r))
		if err != nil || !allowed {
			response.WriteError(
				w,
				mustRouteError(ChallengeResendPostErrors, response.CodeRateLimited),
				"Too many resend attempts. Please try again later.",
			)
			return
		}
	}

	err = h.Challenge.ResendChallenge(r.Context(), challengeID, time.Now().UTC())
	if err != nil && !isOpaqueChallengeResendError(err) {
		response.WriteError(
			w,
			mustRouteError(ChallengeResendPostErrors, response.CodeInternalError),
			"Challenge error.",
		)
		return
	}

	// Keep expected challenge state opaque so resend cannot reveal whether an account exists.
	w.WriteHeader(http.StatusNoContent)
}

func (h *APIHandler) requireChallengeAPI(
	w http.ResponseWriter,
	routeErrors map[response.ErrorCode]response.ErrorSpec,
) bool {
	if !h.ChallengeEnabled {
		response.WriteError(
			w,
			mustRouteError(routeErrors, response.CodeNotFound),
			"Challenge verification is not enabled.",
		)
		return false
	}
	if h.Challenge == nil || h.Verification == nil {
		response.WriteError(
			w,
			mustRouteError(routeErrors, response.CodeInternalError),
			"Challenge error.",
		)
		return false
	}
	return true
}

func writeInvalidChallengeRequest(
	w http.ResponseWriter,
	routeErrors map[response.ErrorCode]response.ErrorSpec,
) {
	response.WriteError(
		w,
		mustRouteError(routeErrors, response.CodeInvalidRequest),
		"Invalid challenge request.",
	)
}

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

func isOpaqueChallengeResendError(err error) bool {
	return errors.Is(err, challenge.ErrChallengeExpired) ||
		errors.Is(err, challenge.ErrChallengeConsumed) ||
		errors.Is(err, challenge.ErrTooManyResends) ||
		errors.Is(err, challenge.ErrResendTooSoon) ||
		errors.Is(err, store.ErrorChallengeNotFound)
}
