package internalapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpmiddleware "github.com/authara-org/authara/internal/http/middleware"
	"github.com/authara-org/authara/internal/organization"
	"github.com/go-chi/chi/v5"
)

func TestCreateOrganizationInvitationRequiresBearerToken(t *testing.T) {
	handler := New(nil)

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

			httpmiddleware.RequireInternalAPIAuth("secret-token")(http.HandlerFunc(handler.CreateOrganizationInvitation)).ServeHTTP(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
			}
		})
	}
}

func TestCreateOrganizationInvitationRequiresActorUserID(t *testing.T) {
	handler := New(nil)

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

func TestCapabilitiesGetReturnsOrganizationMode(t *testing.T) {
	handler := New(organization.New(organization.Config{Mode: organization.OrgModeMulti}))
	req := httptest.NewRequest(http.MethodGet, "/auth/internal/v1/capabilities", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rr := httptest.NewRecorder()

	handler.CapabilitiesGet(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var got struct {
		OrganizationMode          string `json:"organization_mode"`
		AllowsUserCreatedTeamOrgs bool   `json:"allows_user_created_team_orgs"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.OrganizationMode != string(organization.OrgModeMulti) || !got.AllowsUserCreatedTeamOrgs {
		t.Fatalf("unexpected capabilities: %+v", got)
	}
}

func TestCreateOrganizationRequiresCreatedByUserID(t *testing.T) {
	handler := New(nil)
	req := httptest.NewRequest(
		http.MethodPost,
		"/auth/internal/v1/organizations",
		strings.NewReader(`{"name":"Team","created_by_user_id":"not-a-uuid"}`),
	)
	req.Header.Set("Authorization", "Bearer secret-token")
	rr := httptest.NewRecorder()

	handler.CreateOrganization(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}
