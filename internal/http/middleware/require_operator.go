package middleware

import (
	"net/http"

	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/requesterror"
)

// RequireOperator allows only identities with the exact operator platform
// role. Other privileged roles do not imply operator access.
func RequireOperator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		roles, ok := httpctx.Roles(r.Context())
		if !ok || !roles.IsOperator() {
			_ = requesterror.Forbidden(nil, w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}
