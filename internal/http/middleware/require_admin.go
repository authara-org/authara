package middleware

import (
	"net/http"

	httpcontext "github.com/alexlup06-authgate/authgate/internal/http/context"
)

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		roles, ok := httpcontext.Roles(r.Context())
		if !ok || !roles.IsAdmin() {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
