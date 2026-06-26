package internalapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/go-chi/chi/v5"
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

		handler := New(organization.New(organization.Config{Store: tdb.Store, Tx: tdb.Tx}))
		req := httptest.NewRequest(http.MethodGet, "/auth/internal/v1/organizations/"+org.ID.String()+"/members", nil).WithContext(ctx)
		req = withInternalURLParam(req, "organizationID", org.ID.String())
		rr := httptest.NewRecorder()

		handler.ListOrganizationMembers(rr, req)

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

func withInternalURLParam(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}
