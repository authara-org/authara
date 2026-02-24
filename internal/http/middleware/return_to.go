package middleware

import (
	"net/http"

	"github.com/alexlup06-authgate/authgate/internal/http/kit/httpctx"
)

func ReturnTo(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Explicit return_to query parameter (highest priority)
		if rt := r.URL.Query().Get("return_to"); rt != "" {
			if normalized, ok := normalizeReturnTo(rt); ok {
				r = r.WithContext(
					httpctx.WithReturnTo(r.Context(), normalized),
				)
			}
		}

		// 2. Fallback: current request path + query
		if _, ok := httpctx.ReturnTo(r.Context()); !ok {
			current := r.URL.Path
			if r.URL.RawQuery != "" {
				current += "?" + r.URL.RawQuery
			}

			// Prevent auth redirect loops
			if r.URL.Path != "/auth/signup" || r.URL.Path != "/auth/login" {
				r = r.WithContext(
					httpctx.WithReturnTo(r.Context(), current),
				)
			}
		}

		next.ServeHTTP(w, r)
	})
}

func normalizeReturnTo(raw string) (string, bool) {
	if raw == "" {
		return "", false
	}

	if raw[0] != '/' {
		return "", false
	}

	if len(raw) > 1 && raw[1] == '/' {
		return "", false
	}

	return raw, true
}
