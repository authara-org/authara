package middleware

import (
	"net/http"
	"net/url"
	"time"

	httpcontext "github.com/alexlup06-authgate/authgate/internal/http/context"
	"github.com/alexlup06-authgate/authgate/internal/http/redirect"
	"github.com/alexlup06-authgate/authgate/internal/session"
	"github.com/alexlup06-authgate/authgate/internal/session/token"
)

// RequirePageAccessAuthWithRefresh protects AuthGate-rendered pages + HTMX actions.
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
				identity, err := sessionSvc.ValidateAccessToken(ctx, accessToken, now())
				if err == nil {
					ctx = httpcontext.WithUserID(ctx, identity.UserID)
					ctx = httpcontext.WithRoles(ctx, identity.Roles)
					ctx = httpcontext.WithSessionID(ctx, identity.SessionID)
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
					identity, err := sessionSvc.ValidateAccessToken(ctx, newAccess, now())
					if err == nil {
						ctx = httpcontext.WithUserID(ctx, identity.UserID)
						ctx = httpcontext.WithRoles(ctx, identity.Roles)
						ctx = httpcontext.WithSessionID(ctx, identity.SessionID)
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
			returnTo, ok := httpcontext.ReturnTo(ctx)
			if !ok || returnTo == "" {
				returnTo = "/"
			}

			loginURL := "/auth/login?return_to=" + url.QueryEscape(returnTo)
			redirect.Redirect(w, r, loginURL, http.StatusSeeOther)
		})
	}
}
