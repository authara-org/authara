package middleware

import (
	"net/http"

	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/http/kit/requesterror"
)

func RequireAllowlistEnabled(enabled bool) func(http.Handler) http.Handler {
	return RequireAllowlistEnabledWithPolicy(config.AllowlistPolicyReaderFunc(func() config.AllowlistPolicy {
		return config.AllowlistPolicy{AllowlistEnabled: enabled}
	}))
}

func RequireAllowlistEnabledWithPolicy(policy config.AllowlistPolicyReader) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if policy == nil || !policy.CurrentAllowlist().AllowlistEnabled {
				_ = requesterror.NotFound(nil, w, r)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
