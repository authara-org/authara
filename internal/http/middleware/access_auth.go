package middleware

import (
	"net/http"
	"time"

	httpcontext "github.com/alexlup06-authgate/authgate/internal/http/context"
	"github.com/alexlup06-authgate/authgate/internal/session"
)

func RequireAccessAuth(sessionSvc *session.Service, now func() time.Time) func(http.Handler) http.Handler {

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			accessToken, ok := session.ReadAccessToken(r)
			if !ok || accessToken == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			userID, err := sessionSvc.ValidateAccessToken(ctx, accessToken, now())
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			ctx = httpcontext.WithUserID(ctx, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
