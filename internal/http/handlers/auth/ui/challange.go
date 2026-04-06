package ui

import (
	"net/http"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/challenge"
	"github.com/authara-org/authara/internal/domain"
	challengeview "github.com/authara-org/authara/internal/http/templates/challenge"
	"github.com/authara-org/authara/internal/http/templates/components/toast"
	"github.com/google/uuid"
)

func (h *UIHandler) VerifyChallengePage(w http.ResponseWriter, r *http.Request) {
	challengeIDStr := r.URL.Query().Get("challenge_id")
	_ = h.Render(
		w,
		r,
		http.StatusOK,
		challengeview.VerifyChallenge(challengeIDStr, "Verify your Email"),
	)
}

func (h *UIHandler) VerifyChallengePost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	challengeIDStr := strings.TrimSpace(r.FormValue("challenge_id"))
	code := strings.TrimSpace(r.FormValue("code"))

	challengeID, err := uuid.Parse(challengeIDStr)
	if err != nil {
		h.renderFormError(
			w,
			r,
			http.StatusUnprocessableEntity,
			"Invalid verification request.",
			challengeview.VerifyChallengeForm(challengeIDStr, true),
		)
		return
	}

	if len(code) != 6 {
		h.renderFormError(
			w,
			r,
			http.StatusUnprocessableEntity,
			"Please enter the 6-digit verification code.",
			challengeview.VerifyChallengeForm(challengeIDStr, true),
		)
		return
	}

	result, err := h.Challenge.VerifySignupChallenge(
		ctx,
		challengeID,
		code,
		h.Verification,
		time.Now().UTC(),
	)
	if err != nil {
		msg := "Invalid or expired verification code."

		switch err {
		case challenge.ErrChallengeExpired:
			msg = "This verification code has expired."
		case challenge.ErrChallengeConsumed:
			msg = "This verification code has already been used."
		case challenge.ErrTooManyAttempts:
			msg = "Too many incorrect attempts. Please start again."
		case challenge.ErrInvalidVerificationCode:
			msg = "The verification code is incorrect."
		}

		h.renderFormError(
			w,
			r,
			http.StatusUnprocessableEntity,
			msg,
			challengeview.VerifyChallengeForm(challengeIDStr, true),
		)
		return
	}

	h.finishSignup(
		w,
		r,
		auth.SignupInput{
			Provider:     domain.ProviderPassword,
			Username:     result.Action.Username,
			Email:        result.Action.Email,
			PasswordHash: result.Action.PasswordHash,
		},
		challengeview.VerifyChallengeForm(challengeIDStr, true),
	)
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
