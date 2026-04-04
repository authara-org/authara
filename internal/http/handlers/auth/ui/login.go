package ui

import (
	"net/http"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/handlers/auth/ui/flow"
	"github.com/authara-org/authara/internal/http/kit/flash"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/httputil"
	"github.com/authara-org/authara/internal/http/kit/redirect"
	authview "github.com/authara-org/authara/internal/http/templates/auth"
	"github.com/authara-org/authara/internal/session"
)

func (h *UIHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	if flow.TryRedirectAuthenticated(w, r, h.Session, h.AccessTTL, h.RefreshTTL) {
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
		authview.Login(h.OAuthProviders.Providers),
	)
}

func (h *UIHandler) LoginPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		h.renderFormError(w, r, http.StatusBadRequest, "Bad Form", authview.LoginForm())
		return
	}

	email := strings.TrimSpace(r.FormValue("email"))
	email = strings.ToLower(email)
	password := r.FormValue("password")

	if email == "" || password == "" {
		h.renderFormError(w, r, http.StatusBadRequest, "Email and password required.", authview.LoginForm())
		return
	}

	input := auth.LoginInput{
		Provider: domain.ProviderPassword,
		Email:    email,
		Password: password,
	}

	ip := httputil.ClientIP(r)
	allowed, err := h.Limiter.AllowLoginAttempt(ctx, ip, email)
	if err != nil || !allowed {
		h.renderFormError(w, r, http.StatusTooManyRequests, "Too many attempts. Please try again later.", authview.LoginForm())
		return
	}

	user, err := h.Auth.Login(ctx, input)
	if err != nil || user == nil {
		h.renderFormError(w, r, http.StatusUnprocessableEntity, "Invalid email or password.", authview.LoginForm())
		return
	}

	returnTo, ok := httpctx.ReturnTo(r.Context())
	if !ok {
		returnTo = "/"
	}

	audience := redirect.AudienceForPath(returnTo)
	ua := r.UserAgent()
	now := time.Now()
	accessToken, refreshToken, err := h.Session.CreateSession(ctx, user.ID, audience, ua, now)
	if err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}

	session.SetAccessToken(w, accessToken, int(h.AccessTTL.Seconds()))
	session.SetRefreshToken(w, refreshToken, int(h.RefreshTTL.Seconds()))

	redirect.Redirect(w, r, returnTo, http.StatusSeeOther)
}
