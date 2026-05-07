package ui

import (
	"net/http"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/challenge"
	"github.com/authara-org/authara/internal/domain"
	authhandler "github.com/authara-org/authara/internal/http/handlers/auth"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/httputil"
	"github.com/authara-org/authara/internal/http/kit/render"
	authview "github.com/authara-org/authara/internal/http/templates/auth"
	"github.com/authara-org/authara/internal/http/templates/components/toast"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/store"
	"github.com/google/uuid"
)

type passwordResetFormInput struct {
	Email       string
	NewPassword string
}

func (h *UIHandler) PasswordResetPage(w http.ResponseWriter, r *http.Request) {
	_ = h.Render(w, r, http.StatusOK, authview.PasswordReset())
}

func (h *UIHandler) PasswordResetRequestPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	form, err := h.parsePasswordResetForm(r)
	if err != nil {
		h.renderFormError(
			w,
			r,
			http.StatusUnprocessableEntity,
			"Please provide a valid email and password.",
			authview.PasswordResetForm(),
		)
		return
	}

	if !authhandler.IsValidEmail(form.Email) || !authhandler.IsValidPassword(form.NewPassword) {
		h.renderFormError(
			w,
			r,
			http.StatusUnprocessableEntity,
			"Please provide a valid email and password.",
			authview.PasswordResetForm(),
		)
		return
	}

	allowed, err := h.Limiter.AllowPasswordResetAttempt(ctx, httputil.ClientIP(r), form.Email)
	if err != nil || !allowed {
		h.renderFormError(
			w,
			r,
			http.StatusTooManyRequests,
			"Too many reset attempts. Please try again later.",
			authview.PasswordResetForm(),
		)
		return
	}

	passwordHash, err := auth.Hash(form.NewPassword)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var challengeID string

	user, err := h.Auth.GetUserByEmail(ctx, form.Email)
	switch {
	case err == nil:
		id, err := h.Challenge.CreatePasswordResetChallenge(
			ctx,
			challenge.CreatePasswordResetChallengeInput{
				UserID:       user.ID,
				Email:        user.Email,
				PasswordHash: passwordHash,
			},
			time.Now().UTC(),
		)
		if err != nil {
			h.renderFormError(
				w,
				r,
				http.StatusUnprocessableEntity,
				"Could not start password reset. Please try again.",
				authview.PasswordResetForm(),
			)
			return
		}
		challengeID = id.String()

	case err == store.ErrUserNotFound:
		// Opaque challenge to avoid user enumeration.
		id, err := h.Challenge.CreateOpaqueChallenge(
			ctx,
			time.Now().UTC(),
			domain.ChallengePurposePasswordReset,
			form.Email,
		)
		if err != nil {
			h.renderFormError(
				w,
				r,
				http.StatusUnprocessableEntity,
				"Could not start password reset. Please try again.",
				authview.PasswordResetForm(),
			)
			return
		}
		challengeID = id.String()

	default:
		h.renderFormError(
			w,
			r,
			http.StatusUnprocessableEntity,
			"Could not start password reset. Please try again.",
			authview.PasswordResetForm(),
		)
		return
	}

	_ = h.renderVerifyChallengeRedirect(
		w,
		r,
		VerifyChallengeActionPasswordReset,
		challengeID,
		httpctx.ReturnToOrDefault(ctx, "/auth/login"),
	)
}

func (h *UIHandler) parsePasswordResetForm(r *http.Request) (*passwordResetFormInput, error) {
	if err := r.ParseForm(); err != nil {
		return nil, err
	}

	email := strings.TrimSpace(r.FormValue("email"))
	email = strings.ToLower(email)

	return &passwordResetFormInput{
		Email:       email,
		NewPassword: r.FormValue("new_password"),
	}, nil
}

func (h *UIHandler) verifyPasswordResetChallengePost(
	w http.ResponseWriter,
	r *http.Request,
	challengeIDStr string,
	challengeID uuid.UUID,
	code string,
) {
	ctx := r.Context()

	result, err := h.Challenge.VerifyPasswordResetChallenge(
		ctx,
		challengeID,
		code,
		h.Verification,
		time.Now().UTC(),
	)
	if err != nil {
		h.renderVerifyChallengeError(
			w,
			r,
			VerifyChallengeActionPasswordReset,
			challengeIDStr,
			h.verifyChallengeErrorMessage(err),
		)
		return
	}

	if err := h.Challenge.ExecutePasswordReset(ctx, result.Action, time.Now().UTC()); err != nil {
		h.renderVerifyChallengeError(
			w,
			r,
			VerifyChallengeActionPasswordReset,
			challengeIDStr,
			"Could not reset password. Please try again.",
		)
		return
	}

	session.ClearSessionCookies(w)

	c := templ.Join(
		authview.Login(h.OAuthProviders.Providers),
		toast.ToastMessage(toast.Success, "Your password has been reset. Please log in again."),
	)

	_ = render.IntoBody(
		h.Render,
		w,
		r,
		http.StatusOK,
		httpctx.ReturnToOrDefault(ctx, "/auth/login"),
		c,
	)
}
