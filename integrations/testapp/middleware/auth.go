package middleware

import (
	"net/http"
)

func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := r.Cookie("authgate_session")
		if err != nil {
			http.Redirect(w, r, "/auth/login", http.StatusFound)
			return
		}

		next.ServeHTTP(w, r)
	})
}
