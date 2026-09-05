package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/session/roles"
)

func TestRequireOperatorAllowsOperator(t *testing.T) {
	var operatorRoles roles.Roles
	operatorRoles.AddOperator()

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/auth/operator", nil)
	req = req.WithContext(httpctx.WithRoles(req.Context(), operatorRoles))
	rr := httptest.NewRecorder()

	RequireOperator(next).ServeHTTP(rr, req)

	if !called {
		t.Fatal("expected operator request to reach next handler")
	}
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
}

func TestRequireOperatorRejectsOtherRoles(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*roles.Roles)
	}{
		{name: "missing role context"},
		{name: "no roles", setup: func(*roles.Roles) {}},
		{name: "admin", setup: func(r *roles.Roles) { r.AddAdmin() }},
		{name: "auditor", setup: func(r *roles.Roles) { r.AddAuditor() }},
		{name: "monitor", setup: func(r *roles.Roles) { r.AddMonitor() }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				called = true
			})

			req := httptest.NewRequest(http.MethodGet, "/auth/operator", nil)
			if tt.setup != nil {
				var requestRoles roles.Roles
				tt.setup(&requestRoles)
				req = req.WithContext(httpctx.WithRoles(req.Context(), requestRoles))
			}
			rr := httptest.NewRecorder()

			RequireOperator(next).ServeHTTP(rr, req)

			if called {
				t.Fatal("expected request to be rejected")
			}
			if rr.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d", rr.Code, http.StatusForbidden)
			}
		})
	}
}

func TestRequireOperatorAllowsAdditiveOperatorRole(t *testing.T) {
	var requestRoles roles.Roles
	requestRoles.AddAdmin()
	requestRoles.AddOperator()

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/auth/operator", nil)
	req = req.WithContext(httpctx.WithRoles(req.Context(), requestRoles))
	rr := httptest.NewRecorder()

	RequireOperator(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
}
