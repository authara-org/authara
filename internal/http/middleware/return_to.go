package middleware

import (
	"net/http"

	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/redirect"
)

func ReturnTo(next http.Handler) http.Handler {
	return ReturnToWithDefault("/")(next)
}

func ReturnToWithDefault(defaultReturnTo string) func(http.Handler) http.Handler {
	return ReturnToWithPolicy(config.UIPolicyReaderFunc(func() config.UIPolicy {
		return config.UIPolicy{DefaultReturnTo: defaultReturnTo}
	}))
}

func ReturnToWithPolicy(policy config.UIPolicyReader) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defaultReturnTo := "/"
			if policy != nil {
				if normalized, ok := redirect.NormalizeReturnTo(policy.CurrentUI().DefaultReturnTo); ok {
					defaultReturnTo = normalized
				}
			}
			r = r.WithContext(httpctx.WithDefaultReturnTo(r.Context(), defaultReturnTo))

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
}
