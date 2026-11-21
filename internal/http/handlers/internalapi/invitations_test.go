package internalapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/domain"
	httpmiddleware "github.com/authara-org/authara/internal/http/middleware"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/testutil"
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

func TestInternalInvitationManagementAllowsListingRevokingAndReinviting(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		owner, err := tdb.Store.CreateUser(ctx, domain.User{Email: "internal-invite-owner@example.com", Username: "internal-invite-owner"})
		if err != nil {
			t.Fatal(err)
		}
		org, _, err := tdb.Store.EnsureOrganizationForUser(ctx, owner.ID, "Internal Invites", domain.OrganizationKindTeam)
		if err != nil {
			t.Fatal(err)
		}
		svc := organization.New(organization.Config{Store: tdb.Store, Tx: tdb.Tx, Mode: organization.OrgModeMulti, InvitationTTL: time.Hour})
		now := time.Now().UTC()
		created, err := svc.CreateInvitation(ctx, organization.CreateInvitationInput{
			OrganizationID: org.ID,
			ActorUserID:    owner.ID,
			Email:          "reinvite@example.com",
			Now:            now,
		})
		if err != nil {
			t.Fatal(err)
		}
		handler := New(nil, svc, false)

		listResponse, err := handler.ListInternalOrganizationInvitations(ctx, contract.ListInternalOrganizationInvitationsRequestObject{OrganizationID: org.ID})
		if err != nil {
			t.Fatal(err)
		}
		listRecorder := httptest.NewRecorder()
		writeContractResponse(t, listRecorder, listResponse)
		if listRecorder.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", listRecorder.Code, listRecorder.Body.String())
		}
		var listed contract.OrganizationInvitations
		if err := json.Unmarshal(listRecorder.Body.Bytes(), &listed); err != nil {
			t.Fatal(err)
		}
		if len(listed.Invitations) != 1 || listed.Invitations[0].Id != created.Invitation.ID {
			t.Fatalf("unexpected invitations: %+v", listed.Invitations)
		}

		revokeResponse, err := handler.RevokeInternalOrganizationInvitation(ctx, contract.RevokeInternalOrganizationInvitationRequestObject{
			OrganizationID: org.ID,
			InvitationID:   created.Invitation.ID,
		})
		if err != nil {
			t.Fatal(err)
		}
		revokeRecorder := httptest.NewRecorder()
		writeContractResponse(t, revokeRecorder, revokeResponse)
		if revokeRecorder.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", revokeRecorder.Code, revokeRecorder.Body.String())
		}

		if _, err := svc.CreateInvitation(ctx, organization.CreateInvitationInput{
			OrganizationID: org.ID,
			ActorUserID:    owner.ID,
			Email:          created.Invitation.Email,
			Now:            now.Add(time.Minute),
		}); err != nil {
			t.Fatalf("expected a fresh invitation after revocation: %v", err)
		}
	})
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

func TestPublicCapabilitiesReadsCurrentOrganizationPolicy(t *testing.T) {
	enabled := false
	handler := NewWithPolicy(nil, organization.New(organization.Config{Mode: organization.OrgModeMulti}), config.OrganizationPolicyReaderFunc(func() config.OrganizationPolicy {
		return config.OrganizationPolicy{PublicManagementEnabled: enabled}
	}))
	if handler.publicOrganizationManagementEnabled() {
		t.Fatal("public organization management unexpectedly enabled")
	}
	enabled = true
	if !handler.publicOrganizationManagementEnabled() {
		t.Fatal("updated public organization management policy was not observed")
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
