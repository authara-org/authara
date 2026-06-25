package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/session/roles"
	"github.com/authara-org/authara/internal/testutil"
)

func TestUserGetIncludesOrganization(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "api-user-org@example.com",
			Username: "api-user-org",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}
		org, membership, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username)
		if err != nil {
			t.Fatalf("EnsureDefaultOrganizationForUser failed: %v", err)
		}

		h := &APIHandler{
			Auth:          auth.New(auth.Config{Store: tdb.Store, Tx: tdb.Tx}),
			Organizations: organization.New(organization.Config{Store: tdb.Store, Tx: tdb.Tx}),
		}

		reqCtx := httpctx.WithUserID(ctx, user.ID)
		reqCtx = httpctx.WithRoles(reqCtx, roles.Roles{})
		reqCtx = httpctx.WithOrganizationID(reqCtx, org.ID)
		reqCtx = httpctx.WithOrganizationRole(reqCtx, membership.Role)

		req := httptest.NewRequest(http.MethodGet, "/auth/api/v1/user", nil).WithContext(reqCtx)
		rr := httptest.NewRecorder()

		h.UserGet(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var got struct {
			Organization struct {
				ID   string `json:"id"`
				Name string `json:"name"`
				Role string `json:"role"`
			} `json:"organization"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if got.Organization.ID != org.ID.String() {
			t.Fatalf("expected organization id %q, got %q", org.ID, got.Organization.ID)
		}
		if got.Organization.Name != org.Name {
			t.Fatalf("expected organization name %q, got %q", org.Name, got.Organization.Name)
		}
		if got.Organization.Role != string(domain.OrganizationRoleOwner) {
			t.Fatalf("expected organization role owner, got %q", got.Organization.Role)
		}
	})
}
