package ui

import (
	"net/http"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/challenge"
	"github.com/authara-org/authara/internal/http/kit/htmx"
	challengeview "github.com/authara-org/authara/internal/http/templates/challenge"
	"github.com/authara-org/authara/internal/http/templates/components/toast"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type VerifyChallengeAction string

const (
	VerifyChallengeActionSignup        VerifyChallengeAction = "signup"
	VerifyChallengeActionPasswordReset VerifyChallengeAction = "password-reset"
)

func parseVerifyChallengeAction(raw string) (VerifyChallengeAction, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(VerifyChallengeActionSignup):
		return VerifyChallengeActionSignup, true
	case string(VerifyChallengeActionPasswordReset):
		return VerifyChallengeActionPasswordReset, true
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
	default:
		return "Verify your Request"
	}
}

func (h *UIHandler) VerifyChallengePage(w http.ResponseWriter, r *http.Request) {
	challengeIDStr := strings.TrimSpace(r.URL.Query().Get("challenge_id"))

	action, ok := parseVerifyChallengeAction(chi.URLParam(r, "action"))
	if !ok {
		http.Error(w, "invalid verification action", http.StatusBadRequest)
		return
	}

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

	url := "/auth/verify-challenge/" + action.Path() + "?challenge_id=" + challengeID
	if returnTo != "" {
		url += "&return_to=" + returnTo
	}

	htmx.PushUrl(w, url)

	return h.Render(
		w,
		r,
		http.StatusOK,
		challengeview.VerifyChallenge(challengeID, action.Path(), action.Header()),
	)
}

func (h *UIHandler) VerifyChallengePost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	action, ok := parseVerifyChallengeAction(chi.URLParam(r, "action"))
	if !ok {
		http.Error(w, "invalid verification action", http.StatusBadRequest)
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
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	challengeIDStr := strings.TrimSpace(r.FormValue("challenge_id"))
	challengeID, err := uuid.Parse(challengeIDStr)
	if err != nil {
		http.Error(w, "invalid challenge", http.StatusBadRequest)
		return
	}

	err = h.Challenge.ResendChallenge(ctx, challengeID, time.Now().UTC())
	if err != nil {
		msg := "Could not resend verification code."

		switch err {
		case challenge.ErrChallengeExpired:
			msg = "This verification request has expired."
		case challenge.ErrChallengeConsumed:
			msg = "This verification request has already been completed."
		case challenge.ErrTooManyResends:
			msg = "Too many resend attempts. Please start again."
		case challenge.ErrResendTooSoon:
			msg = "Please wait a moment before requesting another code."
		}

		_ = h.Render(
			w,
			r,
			http.StatusOK,
			toast.ToastMessage(toast.Error, msg),
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
