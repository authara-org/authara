package ui

import (
	"context"
	"net/http"
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/flash"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/redirect"
	"github.com/authara-org/authara/internal/session"
)

func (h *UIHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		h.renderError(w, r, ctx)
		return
	}

	idToken := r.FormValue("credential")
	gtoken := r.FormValue("g_csrf_token")

	if idToken == "" || gtoken == "" {
		h.renderError(w, r, ctx)
		return

	}

	csrfCookie, err := r.Cookie("g_csrf_token")
	if err != nil {
		h.renderError(w, r, ctx)
		return

	}

	if gtoken == "" || gtoken != csrfCookie.Value {
		h.renderError(w, r, ctx)
		return

	}

	identity, err := h.Google.VerifyIDToken(ctx, idToken)
	if err != nil {
		h.renderError(w, r, ctx)
		return

	}

	input := auth.LoginInput{
		Provider: domain.ProviderGoogle,
		Email:    identity.Email,
		OAuthID:  identity.OAuthID,
	}

	user, err := h.Auth.Login(ctx, input)
	if err != nil {
		h.renderError(w, r, ctx)
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
		h.renderError(w, r, ctx)
		return

	}

	session.SetAccessToken(w, accessToken, int(h.AccessTTL.Seconds()))
	session.SetRefreshToken(w, refreshToken, int(h.RefreshTTL.Seconds()))

	redirect.Redirect(w, r, returnTo, http.StatusSeeOther)
}

func (h *UIHandler) renderError(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	_ = flash.Set(w, flash.Message{
		Kind:    "error",
		Message: "Google login failed. Please try again.",
	})
	redirect.Redirect(w, r, redirect.WithReturnTo("/auth/login", httpctx.ReturnToOrDefault(ctx, "/")), http.StatusSeeOther)
	return
}
