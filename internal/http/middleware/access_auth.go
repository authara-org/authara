package middleware

import (
	"net/http"
	"time"

	"github.com/alexlup06-authgate/authgate/internal/http/kit/httpctx"
	"github.com/alexlup06-authgate/authgate/internal/session"
	"github.com/alexlup06-authgate/authgate/internal/session/token"
)

func RequireAccessAuth(sessionSvc *session.Service, audience token.Audience, now func() time.Time) func(http.Handler) http.Handler {

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			accessToken, ok := session.ReadAccessToken(r)
			if !ok || accessToken == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			identity, err := sessionSvc.ValidateAccessToken(ctx, accessToken, now())
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			ctx = httpctx.WithUserID(ctx, identity.UserID)
			ctx = httpctx.WithRoles(ctx, identity.Roles)
			ctx = httpctx.WithSessionID(ctx, identity.SessionID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
