package middleware

import (
	"net/http"
	"time"

	httpcontext "github.com/alexlup06-authgate/authgate/internal/http/kit/context"
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

			ctx = httpcontext.WithUserID(ctx, identity.UserID)
			ctx = httpcontext.WithRoles(ctx, identity.Roles)
			ctx = httpcontext.WithSessionID(ctx, identity.SessionID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
