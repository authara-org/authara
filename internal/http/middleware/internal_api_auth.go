package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/authara-org/authara/internal/http/kit/response"
)

func RequireInternalAPIAuth(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !validInternalAPIToken(r, token) {
				response.ErrorJSON(w, http.StatusUnauthorized, response.CodeUnauthorized, "Unauthorized")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func validInternalAPIToken(r *http.Request, token string) bool {
	if strings.TrimSpace(token) == "" {
		return false
	}
	const prefix = "Bearer "
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	got := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	return subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1
}
