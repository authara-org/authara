package middleware

import (
	"net/http"

	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/http/kit/response"
)

func RequirePublicOrganizationManagement(enabled bool) func(http.Handler) http.Handler {
	return RequirePublicOrganizationManagementWithPolicy(config.OrganizationPolicyReaderFunc(func() config.OrganizationPolicy {
		return config.OrganizationPolicy{PublicManagementEnabled: enabled}
	}))
}

func RequirePublicOrganizationManagementWithPolicy(policy config.OrganizationPolicyReader) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if policy == nil || !policy.CurrentOrganization().PublicManagementEnabled {
				response.ErrorJSON(w, http.StatusNotFound, response.CodeNotFound, "Not found")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
