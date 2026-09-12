package ui

import (
	"net/http"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/handlers/ui/flow"
	"github.com/authara-org/authara/internal/http/kit/flash"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/httputil"
	"github.com/authara-org/authara/internal/http/kit/redirect"
	"github.com/authara-org/authara/internal/http/kit/validation"
	authview "github.com/authara-org/authara/internal/http/templates/auth"
	"github.com/authara-org/authara/internal/session"
)

func (h *UIHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	cookiePolicy := h.sessionCookiePolicy()
	if flow.TryRedirectAuthenticated(w, r, h.Session, cookiePolicy.AccessTokenTTL, cookiePolicy.RefreshTokenTTL) {
		return
	}

	msg, _ := flash.Read(w, r)
	if msg != nil {
		r = r.WithContext(httpctx.WithFlash(r.Context(), msg))
	}

	_ = h.Render(
		w,
		r,
		http.StatusOK,
		authview.Login(h.OAuthProviders.Providers, h.usernameLoginEnabled()),
	)
}

func (h *UIHandler) LoginPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	usernameLoginEnabled := h.usernameLoginEnabled()

	if err := r.ParseForm(); err != nil {
		h.renderFormError(w, r, http.StatusBadRequest, "Bad Form", authview.LoginForm(usernameLoginEnabled))
		return
	}

	identifier := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")

	if identifier == "" || password == "" {
		message := "Email and password required."
		if usernameLoginEnabled {
			message = "Email or username and password required."
		}
		h.renderFormError(w, r, http.StatusBadRequest, message, authview.LoginForm(usernameLoginEnabled))
		return
	}

	input := auth.LoginInput{
		Provider: domain.ProviderPassword,
		Email:    strings.ToLower(identifier),
		Password: password,
	}
	invalidCredentialsMessage := "Invalid email or password."
	if usernameLoginEnabled {
		input.Identifier = identifier
		input.Email = ""
		invalidCredentialsMessage = "Invalid email, username, or password."
	} else if !validation.IsValidEmail(input.Email) {
		h.renderFormError(w, r, http.StatusBadRequest, "Please provide a valid email address.", authview.LoginForm(false))
		return
	}

	ip := httputil.ClientIP(r)
	rateLimitIdentifier := strings.ToLower(identifier)
	allowed, err := h.Limiter.AllowLoginAttempt(ctx, ip, rateLimitIdentifier)
	if err != nil || !allowed {
		h.renderFormError(w, r, http.StatusTooManyRequests, "Too many attempts. Please try again later.", authview.LoginForm(usernameLoginEnabled))
		return
	}

	user, err := h.Auth.Login(ctx, input)
	if err != nil {
		h.renderFormError(w, r, http.StatusUnprocessableEntity, invalidCredentialsMessage, authview.LoginForm(usernameLoginEnabled))
		return
	}

	returnTo := httpctx.ReturnToOrDefault(r.Context())

	audience := redirect.AudienceForPath(returnTo)
	ua := r.UserAgent()
	now := time.Now()
	accessToken, refreshToken, err := h.Session.CreateSession(ctx, user.ID, audience, ua, now, httputil.ClientIPString(r))
	if err != nil {
		h.renderFormError(w, r, http.StatusUnprocessableEntity, "This account is disabled.", authview.LoginForm(usernameLoginEnabled))
		return
	}

	cookiePolicy := h.sessionCookiePolicy()
	session.SetAccessToken(w, accessToken, int(cookiePolicy.AccessTokenTTL.Seconds()))
	session.SetRefreshToken(w, refreshToken, int(cookiePolicy.RefreshTokenTTL.Seconds()))

	redirect.Redirect(w, r, returnTo, http.StatusSeeOther)
}
