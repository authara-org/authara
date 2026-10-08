package ui

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/challenge"
	"github.com/authara-org/authara/internal/http/kit/htmx"
	"github.com/authara-org/authara/internal/http/kit/httputil"
	challengeview "github.com/authara-org/authara/internal/http/templates/challenge"
	"github.com/authara-org/authara/internal/http/templates/components/toast"
	"github.com/authara-org/authara/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type VerifyChallengeAction string

const (
	VerifyChallengeActionSignup        VerifyChallengeAction = "signup"
	VerifyChallengeActionPasswordReset VerifyChallengeAction = "password-reset"
	VerifyChallengeActionEmailChange   VerifyChallengeAction = "email-change"
)

func parseVerifyChallengeAction(raw string) (VerifyChallengeAction, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(VerifyChallengeActionSignup):
		return VerifyChallengeActionSignup, true
	case string(VerifyChallengeActionPasswordReset):
		return VerifyChallengeActionPasswordReset, true
	case string(VerifyChallengeActionEmailChange):
		return VerifyChallengeActionEmailChange, true
	default:
		return "", false
	}
}

func (a VerifyChallengeAction) Path() string {
	return string(a)
}

func (a VerifyChallengeAction) Header() string {
	switch a {
	case VerifyChallengeActionSignup:
		return "Verify your Email"
	case VerifyChallengeActionPasswordReset:
		return "Verify your Password Reset"
	case VerifyChallengeActionEmailChange:
		return "Verify your new Email"
	default:
		return "Verify your Request"
	}
}

func (h *UIHandler) VerifyChallengePage(w http.ResponseWriter, r *http.Request) {
	action, ok := parseVerifyChallengeAction(chi.URLParam(r, "action"))
	if !ok {
		h.renderRequestError(w, r, http.StatusBadRequest, "Invalid verification action.")
		return
	}
	if action == VerifyChallengeActionEmailChange {
		h.renderUnauthorized(w, r)
		return
	}

	h.verifyChallengePage(w, r, action)
}

func (h *UIHandler) VerifyEmailChangeChallengePage(w http.ResponseWriter, r *http.Request) {
	h.verifyChallengePage(w, r, VerifyChallengeActionEmailChange)
}

func (h *UIHandler) verifyChallengePage(w http.ResponseWriter, r *http.Request, action VerifyChallengeAction) {
	challengeIDStr := strings.TrimSpace(r.URL.Query().Get("challenge_id"))

	_ = h.Render(
		w,
		r,
		http.StatusOK,
		challengeview.VerifyChallenge(challengeIDStr, action.Path(), action.Header()),
	)
}

func (h *UIHandler) renderVerifyChallengeRedirect(
	w http.ResponseWriter,
	r *http.Request,
	action VerifyChallengeAction,
	challengeID string,
	returnTo string,
) error {
	htmx.ReTarget(w, "#body")
	htmx.ReSwap(w, "innerHTML")

	u := url.URL{Path: "/auth/verify-challenge/" + action.Path()}
	q := u.Query()
	q.Set("challenge_id", challengeID)
	if returnTo != "" {
		q.Set("return_to", returnTo)
	}
	u.RawQuery = q.Encode()
	targetURL := u.String()

	htmx.PushUrl(w, targetURL)

	return h.Render(
		w,
		r,
		http.StatusOK,
		challengeview.VerifyChallenge(challengeID, action.Path(), action.Header()),
	)
}

func (h *UIHandler) VerifyChallengePost(w http.ResponseWriter, r *http.Request) {
	action, ok := parseVerifyChallengeAction(chi.URLParam(r, "action"))
	if !ok {
		h.renderRequestError(w, r, http.StatusBadRequest, "Invalid verification action.")
		return
	}
	if action == VerifyChallengeActionEmailChange {
		h.renderUnauthorized(w, r)
		return
	}

	h.verifyChallengePost(w, r, action)
}

func (h *UIHandler) VerifyEmailChangeChallengePost(w http.ResponseWriter, r *http.Request) {
	h.verifyChallengePost(w, r, VerifyChallengeActionEmailChange)
}

func (h *UIHandler) verifyChallengePost(w http.ResponseWriter, r *http.Request, action VerifyChallengeAction) {
	if err := r.ParseForm(); err != nil {
		h.renderRequestError(w, r, http.StatusBadRequest, "Invalid form.")
		return
	}

	challengeIDStr := strings.TrimSpace(r.FormValue("challenge_id"))
	code := strings.TrimSpace(r.FormValue("code"))

	challengeID, err := uuid.Parse(challengeIDStr)
	if err != nil {
		h.renderVerifyChallengeError(
			w,
			r,
			action,
			challengeIDStr,
			"Invalid verification request.",
		)
		return
	}

	allowed, err := h.Limiter.AllowChallengeVerifyAttempt(r.Context(), httputil.ClientIP(r))
	if status, message, ok := h.rateLimitResult(allowed, err, "Too many verification attempts. Please try again later."); !ok {
		h.renderFormError(
			w,
			r,
			status,
			message,
			challengeview.VerifyChallengeForm(challengeIDStr, action.Path(), true),
		)
		return
	}

	if len(code) != 6 {
		h.renderVerifyChallengeError(
			w,
			r,
			action,
			challengeIDStr,
			"Please enter the 6-digit verification code.",
		)
		return
	}

	switch action {
	case VerifyChallengeActionSignup:
		h.verifySignupChallengePost(w, r, challengeIDStr, challengeID, code)

	case VerifyChallengeActionPasswordReset:
		h.verifyPasswordResetChallengePost(w, r, challengeIDStr, challengeID, code)

	case VerifyChallengeActionEmailChange:
		h.verifyEmailChangeChallengePost(w, r, challengeIDStr, challengeID, code)

	default:
		h.renderVerifyChallengeError(
			w,
			r,
			action,
			challengeIDStr,
			"Unsupported verification request.",
		)
	}
}

func (h *UIHandler) ResendChallengePost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		h.renderRequestError(w, r, http.StatusBadRequest, "Invalid form.")
		return
	}

	challengeIDStr := strings.TrimSpace(r.FormValue("challenge_id"))
	challengeID, err := uuid.Parse(challengeIDStr)
	if err != nil {
		h.renderRequestError(w, r, http.StatusBadRequest, "Invalid challenge.")
		return
	}

	allowed, err := h.Limiter.AllowChallengeResendAttempt(ctx, httputil.ClientIP(r))
	if status, message, ok := h.rateLimitResult(allowed, err, "Too many resend attempts. Please try again later."); !ok {
		_ = h.Render(
			w,
			r,
			status,
			toast.ToastMessage(toast.Error, message),
		)
		return
	}

	err = h.Challenge.ResendChallenge(ctx, challengeID, time.Now().UTC())
	if err != nil && !isExpectedChallengeResendError(err) {
		_ = h.Render(
			w,
			r,
			http.StatusOK,
			toast.ToastMessage(toast.Error, "Could not resend verification code."),
		)
		return
	}

	_ = h.Render(
		w,
		r,
		http.StatusOK,
		toast.ToastMessage(toast.Success, "A new verification code has been sent."),
	)
}

func isExpectedChallengeResendError(err error) bool {
	return errors.Is(err, challenge.ErrChallengeExpired) ||
		errors.Is(err, challenge.ErrChallengeConsumed) ||
		errors.Is(err, challenge.ErrTooManyResends) ||
		errors.Is(err, challenge.ErrResendTooSoon) ||
		errors.Is(err, store.ErrorChallengeNotFound)
}

func (h *UIHandler) renderVerifyChallengeError(
	w http.ResponseWriter,
	r *http.Request,
	action VerifyChallengeAction,
	challengeIDStr string,
	msg string,
) {
	h.renderFormError(
		w,
		r,
		http.StatusUnprocessableEntity,
		msg,
		challengeview.VerifyChallengeForm(challengeIDStr, action.Path(), true),
	)
}

func (h *UIHandler) verifyChallengeErrorMessage(err error) string {
	switch err {
	case challenge.ErrChallengeExpired:
		return "This verification code has expired."
	case challenge.ErrChallengeConsumed:
		return "This verification code has already been used."
	case challenge.ErrTooManyAttempts:
		return "Too many incorrect attempts. Please start again."
	case challenge.ErrInvalidVerificationCode:
		return "The verification code is incorrect."
	default:
		return "Invalid or expired verification code."
	}
}
