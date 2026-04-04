package middleware

import (
	"net/http"

	"github.com/authara-org/authara/internal/http/kit/httpctx"
)

func HTMXMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		ctx := r.Context()
		if "true" == r.Header.Get("HX-Request") {
			ctx = httpctx.WithHTMX(r.Context())
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
