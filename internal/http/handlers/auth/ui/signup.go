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
	"github.com/authara-org/authara/internal/http/handlers/auth/ui/flow"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/httputil"
	"github.com/authara-org/authara/internal/http/kit/redirect"
	authview "github.com/authara-org/authara/internal/http/templates/auth"
	challengeview "github.com/authara-org/authara/internal/http/templates/challenge"
	"github.com/authara-org/authara/internal/session"
	"github.com/google/uuid"
)

func (h *UIHandler) SignupPage(w http.ResponseWriter, r *http.Request) {
	if flow.TryRedirectAuthenticated(w, r, h.Session, h.AccessTTL, h.RefreshTTL) {
		return
	}

	_ = h.Render(
		w,
		r,
		http.StatusOK,
		authview.Signup(h.OAuthProviders.Providers),
	)
}

type signupFormInput struct {
	Email    string
	Password string
}

func (h *UIHandler) SignupPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	form, err := h.parseSignupForm(r)
	if err != nil {
		h.renderFormError(w, r, http.StatusUnprocessableEntity, "Please provide a valid email and password.", authview.SignupForm())
		return
	}

	if !authhandler.IsValidEmail(form.Email) || !authhandler.IsValidPassword(form.Password) {
		h.renderFormError(w, r, http.StatusUnprocessableEntity, "Please provide a valid email and password.", authview.SignupForm())
		return
	}

	ip := httputil.ClientIP(r)
	allowed, err := h.Limiter.AllowSignupAttempt(ctx, ip, form.Email)
	if err != nil || !allowed {
		h.renderFormError(w, r, http.StatusTooManyRequests, "Too many attempts. Please try again later.", authview.SignupForm())
		return
	}

	if h.ChallengeEnabled {
		h.startSignupChallenge(w, r, form.Email, form.Password)
		return
	}

	if h.renderExternalOnlySignupCollision(w, r, form.Email, authview.SignupForm()) {
		return
	}

	passwordHash, err := auth.Hash(form.Password)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.finishSignup(
		w,
		r,
		auth.SignupInput{
			Provider:     domain.ProviderPassword,
			Email:        form.Email,
			PasswordHash: passwordHash,
		},
		authview.SignupForm(),
	)
}

func (h *UIHandler) parseSignupForm(r *http.Request) (*signupFormInput, error) {
	if err := r.ParseForm(); err != nil {
		return nil, err
	}

	email := strings.TrimSpace(r.FormValue("email"))
	email = strings.ToLower(email)

	return &signupFormInput{
		Email:    email,
		Password: r.FormValue("password"),
	}, nil
}

func (h *UIHandler) startSignupChallenge(
	w http.ResponseWriter,
	r *http.Request,
	email string,
	password string,
) {
	ctx := r.Context()

	passwordHash, err := auth.Hash(password)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	exists, err := h.Auth.UserExistsByEmail(ctx, email)
	if err != nil {
		h.renderFormError(
			w, r,
			http.StatusUnprocessableEntity,
			"Could not start signup verification. Please try again.",
			authview.SignupForm(),
		)
		return
	}
	if exists {
		if h.renderExternalOnlySignupCollision(w, r, email, authview.SignupForm()) {
			return
		}

		// Keep signup opaque for existing emails: render the same verification flow
		// for password-backed accounts without creating a pending action or code.
		challengeID, err := h.Challenge.CreateOpaqueChallenge(ctx, time.Now().UTC(), domain.ChallengePurposeSignup, email)
		if err != nil {
			h.renderFormError(
				w, r,
				http.StatusUnprocessableEntity,
				"Could not start signup verification. Please try again.",
				authview.SignupForm(),
			)
			return
		}

		_ = h.renderVerifyChallengeRedirect(
			w,
			r,
			VerifyChallengeActionSignup,
			challengeID.String(),
			httpctx.ReturnToOrDefault(ctx, "/"),
		)
		return
	}

	challengeID, err := h.Challenge.CreateSignupChallenge(ctx, challenge.CreateSignupChallengeInput{
		Email:        email,
		Username:     "",
		PasswordHash: passwordHash,
	}, time.Now().UTC())
	if err != nil {
		h.renderFormError(w, r, http.StatusUnprocessableEntity, "Could not start signup verification. Please try again.", authview.SignupForm())
		return
	}

	_ = h.renderVerifyChallengeRedirect(
		w,
		r,
		VerifyChallengeActionSignup,
		challengeID.String(),
		httpctx.ReturnToOrDefault(ctx, "/"),
	)
}

func (h *UIHandler) renderExternalOnlySignupCollision(
	w http.ResponseWriter,
	r *http.Request,
	email string,
	errorRenderForm templ.Component,
) bool {
	user, err := h.Auth.GetUserByEmail(r.Context(), email)
	if err != nil {
		return false
	}

	providers, err := h.Auth.ListUserAuthProviders(r.Context(), user.ID)
	if err != nil {
		return false
	}

	hasPassword := false
	hasExternalProvider := false
	for _, provider := range providers {
		if provider.Provider == domain.ProviderPassword {
			hasPassword = true
			continue
		}
		hasExternalProvider = true
	}

	if hasPassword || !hasExternalProvider {
		return false
	}

	h.renderFormError(
		w,
		r,
		http.StatusUnprocessableEntity,
		"An account already exists with this email using a social sign-in provider. Sign in with that provider first to add a password.",
		errorRenderForm,
	)
	return true
}

func (h *UIHandler) finishSignup(
	w http.ResponseWriter,
	r *http.Request,
	input auth.SignupInput,
	errorRenderForm templ.Component,
) {
	ctx := r.Context()

	user, err := h.Auth.Signup(ctx, input)
	if err != nil {
		h.renderFormError(
			w,
			r,
			http.StatusUnprocessableEntity,
			"Could not create account. Please check your details.",
			errorRenderForm,
		)
		return
	}

	returnTo, ok := httpctx.ReturnTo(ctx)
	if !ok {
		returnTo = "/"
	}

	audience := redirect.AudienceForPath(returnTo)
	ua := r.UserAgent()
	now := time.Now()

	accessToken, refreshToken, err := h.Session.CreateSession(ctx, user.ID, audience, ua, now)
	if err != nil {
		h.renderFormError(
			w,
			r,
			http.StatusUnprocessableEntity,
			"Did not create session.",
			errorRenderForm,
		)
		return
	}

	session.SetAccessToken(w, accessToken, int(h.AccessTTL.Seconds()))
	session.SetRefreshToken(w, refreshToken, int(h.RefreshTTL.Seconds()))

	redirect.Redirect(w, r, returnTo, http.StatusSeeOther)
}

func (h *UIHandler) verifySignupChallengePost(
	w http.ResponseWriter,
	r *http.Request,
	challengeIDStr string,
	challengeID uuid.UUID,
	code string,
) {
	ctx := r.Context()

	result, err := h.Challenge.VerifySignupChallenge(
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
			VerifyChallengeActionSignup,
			challengeIDStr,
			h.verifyChallengeErrorMessage(err),
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
		challengeview.VerifyChallengeForm(
			challengeIDStr,
			VerifyChallengeActionSignup.Path(),
			true,
		),
	)
}
