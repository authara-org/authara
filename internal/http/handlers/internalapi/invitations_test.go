package internalapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	httpmiddleware "github.com/authara-org/authara/internal/http/middleware"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/organization"
	"github.com/google/uuid"
)

func TestCreateOrganizationInvitationRequiresBearerToken(t *testing.T) {
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

			httpmiddleware.RequireInternalAPIAuth("secret-token")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})).ServeHTTP(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
			}
		})
	}
}

func TestCreateOrganizationInvitationRequiresActorUserID(t *testing.T) {
	handler := New(nil, nil, false)
	organizationID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	rr := httptest.NewRecorder()
	resp, err := handler.CreateInternalOrganizationInvitation(context.Background(), contract.CreateInternalOrganizationInvitationRequestObject{
		OrganizationID: organizationID,
	})
	if err != nil {
		t.Fatalf("CreateInternalOrganizationInvitation failed: %v", err)
	}
	writeContractResponse(t, rr, resp)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestPublicCapabilitiesGetReturnsOrganizationMode(t *testing.T) {
	handler := New(nil, organization.New(organization.Config{Mode: organization.OrgModeMulti}), true)
	req := httptest.NewRequest(http.MethodGet, "/auth/api/v1/capabilities", nil)
	rr := httptest.NewRecorder()

	resp, err := handler.GetPublicCapabilities(req.Context(), contract.GetPublicCapabilitiesRequestObject{})
	if err != nil {
		t.Fatalf("GetPublicCapabilities failed: %v", err)
	}
	writeContractResponse(t, rr, resp)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var got struct {
		OrganizationMode                   string `json:"organization_mode"`
		AllowsUserCreatedTeamOrgs          bool   `json:"allows_user_created_team_orgs"`
		AllowsPublicOrganizationManagement bool   `json:"allows_public_organization_management"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.OrganizationMode != string(organization.OrgModeMulti) ||
		!got.AllowsUserCreatedTeamOrgs ||
		!got.AllowsPublicOrganizationManagement {
		t.Fatalf("unexpected capabilities: %+v", got)
	}
}

func TestCreateOrganizationRequiresCreatedByUserID(t *testing.T) {
	handler := New(nil, nil, false)
	rr := httptest.NewRecorder()

	resp, err := handler.CreateInternalOrganization(context.Background(), contract.CreateInternalOrganizationRequestObject{})
	if err != nil {
		t.Fatalf("CreateInternalOrganization failed: %v", err)
	}
	writeContractResponse(t, rr, resp)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}
