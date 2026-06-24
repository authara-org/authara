package internalapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestCreateOrganizationInvitationRequiresBearerToken(t *testing.T) {
	handler := New(nil, "secret-token")

	for _, tc := range []struct {
		name   string
		header string
	}{
		{name: "missing"},
		{name: "invalid", header: "Bearer wrong-token"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/auth/internal/v1/organizations/11111111-1111-1111-1111-111111111111/invitations", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rr := httptest.NewRecorder()

			handler.CreateOrganizationInvitation(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
			}
		})
	}
}

func TestCreateOrganizationInvitationRequiresActorUserID(t *testing.T) {
	handler := New(nil, "secret-token")

	for _, body := range []string{
		`{"email":"teammate@example.com"}`,
		`{"actor_user_id":"","email":"teammate@example.com"}`,
		`{"actor_user_id":"not-a-uuid","email":"teammate@example.com"}`,
	} {
		req := httptest.NewRequest(
			http.MethodPost,
			"/auth/internal/v1/organizations/11111111-1111-1111-1111-111111111111/invitations",
			strings.NewReader(body),
		)
		req.Header.Set("Authorization", "Bearer secret-token")
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("organizationID", "11111111-1111-1111-1111-111111111111")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		rr := httptest.NewRecorder()

		handler.CreateOrganizationInvitation(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d for body %s", http.StatusBadRequest, rr.Code, body)
		}
	}
}
