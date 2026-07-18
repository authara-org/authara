package middleware

import (
	"net/http"

	"github.com/authara-org/authara/internal/http/kit/response"
)

func RequirePublicOrganizationManagement(enabled bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !enabled {
				response.ErrorJSON(w, http.StatusNotFound, response.CodeNotFound, "Not found")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
