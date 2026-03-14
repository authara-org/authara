package middleware

import (
	"net/http"

	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/redirect"
)

func ReturnTo(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Explicit return_to query parameter (highest priority)
		rt := redirect.QueryParam(r)
		if rt != "" {
			if normalized, ok := redirect.NormalizeReturnTo(rt); ok {
				r = r.WithContext(
					httpctx.WithReturnTo(r.Context(), normalized),
				)
			}
		}

		next.ServeHTTP(w, r)
	})
}
