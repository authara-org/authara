package internalapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/google/uuid"
)

func TestListOrganizationMembersIncludesUserFields(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		owner, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "internal-org-owner@example.com",
			Username: "internal-org-owner",
		})
		if err != nil {
			t.Fatalf("CreateUser owner failed: %v", err)
		}
		org, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, owner.ID, owner.Username)
		if err != nil {
			t.Fatalf("EnsureDefaultOrganizationForUser failed: %v", err)
		}
		teammate, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "internal-org-member@example.com",
			Username: "internal-org-member",
		})
		if err != nil {
			t.Fatalf("CreateUser teammate failed: %v", err)
		}
		if _, err := tdb.Store.CreateOrganizationMembership(ctx, domain.OrganizationMembership{
			OrganizationID: org.ID,
			UserID:         teammate.ID,
			Role:           domain.OrganizationRoleAdmin,
		}); err != nil {
			t.Fatalf("CreateOrganizationMembership failed: %v", err)
		}
		if err := tdb.Store.DisableUser(ctx, teammate.ID, time.Now().UTC()); err != nil {
			t.Fatalf("DisableUser failed: %v", err)
		}

		handler := New(organization.New(organization.Config{
			Store: tdb.Store,
			Tx:    tdb.Tx,
			Mode:  organization.OrgModeMulti,
		}), false)
		reqCtx := httpctx.WithUserID(ctx, owner.ID)
		reqCtx = httpctx.WithOrganizationID(reqCtx, org.ID)
		req := httptest.NewRequest(http.MethodGet, "/auth/api/v1/organizations/"+org.ID.String()+"/members", nil).WithContext(reqCtx)
		rr := httptest.NewRecorder()

		resp, err := handler.ListPublicOrganizationMembers(req.Context(), contract.ListPublicOrganizationMembersRequestObject{
			OrganizationID: org.ID,
		})
		if err != nil {
			t.Fatalf("ListPublicOrganizationMembers failed: %v", err)
		}
		writeContractResponse(t, rr, resp)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var got struct {
			Members []struct {
				OrganizationID string `json:"organization_id"`
				UserID         string `json:"user_id"`
				Email          string `json:"email"`
				Username       string `json:"username"`
				Role           string `json:"role"`
				CreatedAt      string `json:"created_at"`
				UpdatedAt      string `json:"updated_at"`
				Disabled       bool   `json:"disabled"`
			} `json:"members"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		for _, member := range got.Members {
			if member.UserID == teammate.ID.String() {
				if member.OrganizationID != org.ID.String() ||
					member.Email != teammate.Email ||
					member.Username != teammate.Username ||
					member.Role != string(domain.OrganizationRoleAdmin) ||
					member.CreatedAt == "" ||
					member.UpdatedAt == "" ||
					!member.Disabled {
					t.Fatalf("unexpected teammate member: %+v", member)
				}
				return
			}
		}
		t.Fatalf("expected teammate member in %+v", got.Members)
	})
}

func TestPublicOrganizationAuthorizationUsesCurrentMembership(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		owner, err := tdb.Store.CreateUser(ctx, domain.User{Email: "public-org-owner@example.com", Username: "public-org-owner"})
		if err != nil {
			t.Fatalf("CreateUser owner failed: %v", err)
		}
		org, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, owner.ID, owner.Username)
		if err != nil {
			t.Fatalf("EnsureDefaultOrganizationForUser failed: %v", err)
		}
		member, err := tdb.Store.CreateUser(ctx, domain.User{Email: "public-org-member@example.com", Username: "public-org-member"})
		if err != nil {
			t.Fatalf("CreateUser member failed: %v", err)
		}
		if _, err := tdb.Store.CreateOrganizationMembership(ctx, domain.OrganizationMembership{
			OrganizationID: org.ID,
			UserID:         member.ID,
			Role:           domain.OrganizationRoleMember,
		}); err != nil {
			t.Fatalf("CreateOrganizationMembership failed: %v", err)
		}

		handler := New(organization.New(organization.Config{Store: tdb.Store, Tx: tdb.Tx}), true)
		for _, tc := range []struct {
			name           string
			userID         uuid.UUID
			currentOrgID   uuid.UUID
			managerOnly    bool
			wantAuthorized bool
		}{
			{name: "owner may manage", userID: owner.ID, currentOrgID: org.ID, managerOnly: true, wantAuthorized: true},
			{name: "member may read", userID: member.ID, currentOrgID: org.ID, wantAuthorized: true},
			{name: "member may not manage", userID: member.ID, currentOrgID: org.ID, managerOnly: true},
			{name: "different current organization is rejected", userID: owner.ID, currentOrgID: uuid.New()},
		} {
			t.Run(tc.name, func(t *testing.T) {
				reqCtx := httpctx.WithUserID(ctx, tc.userID)
				reqCtx = httpctx.WithOrganizationID(reqCtx, tc.currentOrgID)
				req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(reqCtx)
				rr := httptest.NewRecorder()

				_, resp, authorized := handler.contractAuthorizePublicOrganization(req.Context(), org.ID, GetPublicOrganizationErrors, tc.managerOnly)
				if !authorized {
					writeContractResponse(t, rr, resp)
				}

				if authorized != tc.wantAuthorized {
					t.Fatalf("expected authorized=%v, got %v", tc.wantAuthorized, authorized)
				}
				if !authorized && rr.Code != http.StatusForbidden {
					t.Fatalf("expected status %d, got %d", http.StatusForbidden, rr.Code)
				}
			})
		}
	})
}
