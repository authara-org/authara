package middleware

import (
	"net/http"
	"time"

	"github.com/alexlup06/authgate/internal/session"
)

func RedirectIfAuthenticated(
	sessionService *session.Service,
	now func() time.Time,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only guard UI pages
			if r.Method != http.MethodGet {
				next.ServeHTTP(w, r)
				return
			}

			access, ok := session.ReadAccessToken(r)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			_, err := sessionService.ValidateAccessToken(
				r.Context(),
				access,
				now(),
			)
			if err == nil {
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
