package ui

import (
	"net/http"
	"time"

	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/redirect"
	"github.com/authara-org/authara/internal/session"
)

func (h *UIHandler) LogoutPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	refreshToken, exists := session.ReadRefreshToken(r)
	if exists {
		_ = h.Session.Logout(ctx, refreshToken)
	}

	session.ClearSessionCookies(w)

	returnTo, ok := httpctx.ReturnTo(r.Context())
	if !ok {
		returnTo = "/"
	}

	redirect.Redirect(w, r, returnTo, http.StatusSeeOther)
}

func (h *UIHandler) RefreshPost(w http.ResponseWriter, r *http.Request) {
	now := time.Now()

	returnTo, ok := httpctx.ReturnTo(r.Context())
	if !ok {
		returnTo = r.URL.Path
		if r.URL.RawQuery != "" {
			returnTo += "?" + r.URL.RawQuery
		}
	}

	audience := redirect.AudienceForPath(returnTo)
	refresh, ok := session.ReadRefreshToken(r)
	if !ok {
		session.ClearSessionCookies(w)
		redirect.Redirect(w, r, redirect.WithReturnTo("/auth/login", returnTo), http.StatusSeeOther)
		return
	}

	accessToken, newRefreshToken, err := h.Session.RefreshSession(
		r.Context(),
		refresh,
		audience,
		now,
	)
	if err != nil {
		session.ClearSessionCookies(w)
		redirect.Redirect(w, r, redirect.WithReturnTo("/auth/login", returnTo), http.StatusSeeOther)
		return
	}

	session.SetAccessToken(w, accessToken, int(h.AccessTTL.Seconds()))
	session.SetRefreshToken(w, newRefreshToken, int(h.RefreshTTL.Seconds()))
	redirect.Redirect(w, r, returnTo, http.StatusSeeOther)
}
