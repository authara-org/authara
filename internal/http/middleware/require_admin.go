package middleware

import (
	"net/http"

	"github.com/authara-org/authara/internal/http/kit/httpctx"
)

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		roles, ok := httpctx.Roles(r.Context())
		if !ok || !roles.IsAdmin() {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
