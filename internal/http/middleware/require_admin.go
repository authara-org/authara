package middleware

import (
	"net/http"

	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/requesterror"
)

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		roles, ok := httpctx.Roles(r.Context())
		if !ok || !roles.IsAdmin() {
			_ = requesterror.Forbidden(nil, w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}
