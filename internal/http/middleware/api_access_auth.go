package middleware

import (
	"net/http"
	"time"

	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/response"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/session/token"
)

func RequireAPIAccessAuth(sessionSvc *session.Service, audience token.Audience, now func() time.Time) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			accessToken, ok := session.ReadAccessToken(r)
			if !ok || accessToken == "" {
				response.ErrorJSON(
					w,
					http.StatusUnauthorized,
					response.CodeUnauthorized,
					"Unauthorized",
				)
				return
			}

			identity, err := sessionSvc.ValidateAccessToken(accessToken, audience, now())
			if err != nil {
				response.ErrorJSON(
					w,
					http.StatusUnauthorized,
					response.CodeUnauthorized,
					"Unauthorized",
				)
				return
			}

			ctx = httpctx.WithUserID(ctx, identity.UserID)
			ctx = httpctx.WithRoles(ctx, identity.Roles)
			ctx = httpctx.WithSessionID(ctx, identity.SessionID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
