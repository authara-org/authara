package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequirePublicOrganizationManagement(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	for _, tc := range []struct {
		name    string
		enabled bool
		status  int
	}{
		{name: "enabled", enabled: true, status: http.StatusNoContent},
		{name: "disabled", status: http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/auth/api/v1/organizations/id", nil)

			RequirePublicOrganizationManagement(tc.enabled)(next).ServeHTTP(rr, req)

			if rr.Code != tc.status {
				t.Fatalf("expected status %d, got %d", tc.status, rr.Code)
			}
		})
	}
}
