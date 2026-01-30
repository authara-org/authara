package middleware

import (
	"net/http"
	"time"

	httpcontext "github.com/alexlup06-authgate/authgate/internal/http/context"
	"github.com/alexlup06-authgate/authgate/internal/http/csrf"
	"github.com/alexlup06-authgate/authgate/internal/http/redirect"
	"github.com/alexlup06-authgate/authgate/internal/session"
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
				returnTo, ok := httpcontext.ReturnTo(r.Context())
				if !ok {
					returnTo = "/"
				}

				_, err = csrf.EnsureCookie(w, r)
				if err != nil {
					http.Error(w, "server error", http.StatusInternalServerError)
					return
				}

				redirect.Redirect(w, r, returnTo, http.StatusSeeOther)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
