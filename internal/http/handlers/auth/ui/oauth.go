package ui

import (
	"net/http"
	"time"

	"github.com/alexlup06-authgate/authgate/internal/auth"
	"github.com/alexlup06-authgate/authgate/internal/domain"
	httpcontext "github.com/alexlup06-authgate/authgate/internal/http/kit/context"
	"github.com/alexlup06-authgate/authgate/internal/http/kit/redirect"
	"github.com/alexlup06-authgate/authgate/internal/session"
)

func (h *UIHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	idToken := r.FormValue("credential")
	gtoken := r.FormValue("g_csrf_token")

	if idToken == "" || gtoken == "" {
		http.Error(w, "tokens are empty", http.StatusBadRequest)
		return
	}

	csrfCookie, err := r.Cookie("g_csrf_token")
	if err != nil {
		http.Error(w, "invalid csfr cookie", http.StatusUnauthorized)
		return
	}

	if gtoken == "" || gtoken != csrfCookie.Value {
		http.Error(w, "invalid g_csfr_token", http.StatusUnauthorized)
		return
	}

	identity, err := h.Google.VerifyIDToken(ctx, idToken)
	if err != nil {
		http.Error(w, "invalid id token", http.StatusUnauthorized)
		return
	}

	input := auth.LoginInput{
		Provider: domain.ProviderGoogle,
		Email:    identity.Email,
		OAuthID:  identity.OAuthID,
	}

	user, err := h.Auth.Login(ctx, input)
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	returnTo, ok := httpcontext.ReturnTo(r.Context())
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
