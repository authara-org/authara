package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/authara-org/authara/internal/config"
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

func TestRequirePublicOrganizationManagementReadsCurrentPolicy(t *testing.T) {
	enabled := false
	policy := config.OrganizationPolicyReaderFunc(func() config.OrganizationPolicy {
		return config.OrganizationPolicy{PublicManagementEnabled: enabled}
	})
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	handler := RequirePublicOrganizationManagementWithPolicy(policy)(next)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("disabled status = %d", rr.Code)
	}

	enabled = true
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusNoContent {
		t.Fatalf("enabled status = %d", rr.Code)
	}
}
