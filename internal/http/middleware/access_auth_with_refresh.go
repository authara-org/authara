package middleware

import (
	"net/http"
	"time"

	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/redirect"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/session/token"
)

// RequirePageAccessAuthWithRefresh protects Authara-rendered pages + HTMX actions.
// It validates the access token, and if it's missing/expired it will try to refresh
// using the refresh token cookie. If refresh fails, it redirects to login.
func RequireAccessAuthWithRefresh(
	sessionSvc *session.Service,
	audience token.Audience,
	accessTTL time.Duration,
	refreshTTL time.Duration,
	now func() time.Time,
) func(http.Handler) http.Handler {

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Try access token first
			if accessToken, ok := session.ReadAccessToken(r); ok && accessToken != "" {
				identity, err := sessionSvc.ValidateAccessToken(accessToken, audience, now())
				if err == nil {
					ctx = httpctx.WithUserID(ctx, identity.UserID)
					ctx = httpctx.WithRoles(ctx, identity.Roles)
					ctx = httpctx.WithSessionID(ctx, identity.SessionID)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			// Access missing/invalid -> try refresh token
			refreshToken, ok := session.ReadRefreshToken(r)
			if ok && refreshToken != "" {
				newAccess, newRefresh, err := sessionSvc.RefreshSession(
					ctx,
					refreshToken,
					audience,
					now(),
				)
				if err == nil {
					// set rotated cookies
					session.SetAccessToken(w, newAccess, int(accessTTL.Seconds()))
					session.SetRefreshToken(w, newRefresh, int(refreshTTL.Seconds()))

					// populate context from new access token
					identity, err := sessionSvc.ValidateAccessToken(newAccess, audience, now())
					if err == nil {
						ctx = httpctx.WithUserID(ctx, identity.UserID)
						ctx = httpctx.WithRoles(ctx, identity.Roles)
						ctx = httpctx.WithSessionID(ctx, identity.SessionID)
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}

					// should not happen, but fail safe
					session.ClearSessionCookies(w)
				} else {
					// refresh invalid/reused/expired/etc.
					session.ClearSessionCookies(w)
				}
			}

			// 3) Not authenticated -> redirect to login (HTMX-safe)
			returnTo, ok := httpctx.ReturnTo(ctx)
			if !ok || returnTo == "" {
				returnTo = "/"
			}

			loginURL := redirect.WithReturnTo("/auth/login", returnTo)
			redirect.Redirect(w, r, loginURL, http.StatusSeeOther)
		})
	}
}
