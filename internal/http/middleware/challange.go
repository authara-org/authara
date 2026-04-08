package middleware

import "net/http"

func RequireChallengeEnabled(enabled bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !enabled {
				http.NotFound(w, r)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
