package middleware

import (
	"net/http"

	"github.com/authara-org/authara/internal/http/kit/requesterror"
)

func RequireChallengeEnabled(enabled bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !enabled {
				_ = requesterror.NotFound(nil, w, r)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
