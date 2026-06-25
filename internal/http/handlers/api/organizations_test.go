package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/session/token"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/go-chi/chi/v5"
)

func TestOrganizationsGetAndCurrentGet(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user, personalOrg, personalMembership, teamOrg, teamMembership := createAPIOrganizationUser(t, ctx, tdb)
		h := &APIHandler{
			Organizations: organization.New(organization.Config{Store: tdb.Store, Tx: tdb.Tx}),
		}

		reqCtx := httpctx.WithUserID(ctx, user.ID)
		req := httptest.NewRequest(http.MethodGet, "/auth/api/v1/organizations", nil).WithContext(reqCtx)
		rr := httptest.NewRecorder()

		h.OrganizationsGet(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var list struct {
			Organizations []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
				Role string `json:"role"`
			} `json:"organizations"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &list); err != nil {
			t.Fatalf("decode organizations body: %v", err)
		}
		if len(list.Organizations) != 2 {
			t.Fatalf("expected 2 organizations, got %d", len(list.Organizations))
		}
		assertOrganizationListContains(t, list.Organizations, personalOrg.ID.String(), personalOrg.Name, string(personalMembership.Role))
		assertOrganizationListContains(t, list.Organizations, teamOrg.ID.String(), teamOrg.Name, string(teamMembership.Role))

		currentCtx := httpctx.WithOrganizationID(ctx, teamOrg.ID)
		currentCtx = httpctx.WithOrganizationRole(currentCtx, teamMembership.Role)
		currentReq := httptest.NewRequest(http.MethodGet, "/auth/api/v1/organizations/current", nil).WithContext(currentCtx)
		currentRR := httptest.NewRecorder()

		h.OrganizationCurrentGet(currentRR, currentReq)

		if currentRR.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, currentRR.Code, currentRR.Body.String())
		}

		var current struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Role string `json:"role"`
		}
		if err := json.Unmarshal(currentRR.Body.Bytes(), &current); err != nil {
			t.Fatalf("decode current body: %v", err)
		}
		assertOrganizationJSON(t, current, teamOrg.ID.String(), teamOrg.Name, string(teamMembership.Role))
	})
}

func TestOrganizationSwitchPostReturnsSwitchedTokens(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user, _, _, teamOrg, teamMembership := createAPIOrganizationUser(t, ctx, tdb)
		sessionService := newAPIHandlerTestSessionService(t, tdb)
		now := time.Now().UTC()

		accessToken, _, err := sessionService.CreateSession(ctx, user.ID, token.AudienceApp, "test-agent", now)
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}
		identity, err := sessionService.ValidateAccessToken(accessToken, token.AudienceApp, now)
		if err != nil {
			t.Fatalf("ValidateAccessToken failed: %v", err)
		}

		h := &APIHandler{
			Session:       sessionService,
			Organizations: organization.New(organization.Config{Store: tdb.Store, Tx: tdb.Tx, Mode: organization.OrgModeMulti}),
			AccessTTL:     time.Minute,
			RefreshTTL:    time.Hour,
		}

		reqCtx := httpctx.WithUserID(ctx, user.ID)
		reqCtx = httpctx.WithSessionID(reqCtx, identity.SessionID)
		req := httptest.NewRequest(http.MethodPost, "/auth/api/v1/organizations/"+teamOrg.ID.String()+"/switch", nil).WithContext(reqCtx)
		req = withAPIURLParam(req, "organizationID", teamOrg.ID.String())
		rr := httptest.NewRecorder()

		h.OrganizationSwitchPost(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rr.Code, rr.Body.String())
		}
		if !hasCookie(rr.Result().Cookies(), "authara_access") || !hasCookie(rr.Result().Cookies(), "authara_refresh") {
			t.Fatal("expected switch to set session cookies")
		}

		var got tokensResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode switch body: %v", err)
		}
		if got.AccessToken == "" || got.RefreshToken == "" {
			t.Fatalf("expected response tokens, got %+v", got)
		}

		switchedIdentity, err := sessionService.ValidateAccessToken(got.AccessToken, token.AudienceApp, time.Now())
		if err != nil {
			t.Fatalf("ValidateAccessToken switched failed: %v", err)
		}
		if switchedIdentity.OrganizationID != teamOrg.ID {
			t.Fatalf("expected organization %q, got %q", teamOrg.ID, switchedIdentity.OrganizationID)
		}
		if switchedIdentity.OrganizationRole != teamMembership.Role {
			t.Fatalf("expected role %q, got %q", teamMembership.Role, switchedIdentity.OrganizationRole)
		}
	})
}

func createAPIOrganizationUser(t *testing.T, ctx context.Context, tdb *testutil.TestDB) (domain.User, domain.Organization, domain.OrganizationMembership, domain.Organization, domain.OrganizationMembership) {
	t.Helper()

	user, err := tdb.Store.CreateUser(ctx, domain.User{
		Email:    "api-orgs@example.com",
		Username: "api-orgs",
	})
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	personalOrg, personalMembership, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username)
	if err != nil {
		t.Fatalf("EnsureDefaultOrganizationForUser failed: %v", err)
	}

	createdBy := user.ID
	teamOrg, err := tdb.Store.CreateOrganization(ctx, domain.Organization{
		Name:            "API Team",
		Kind:            domain.OrganizationKindTeam,
		CreatedByUserID: &createdBy,
	})
	if err != nil {
		t.Fatalf("CreateOrganization failed: %v", err)
	}
	teamMembership, err := tdb.Store.CreateOrganizationMembership(ctx, domain.OrganizationMembership{
		OrganizationID: teamOrg.ID,
		UserID:         user.ID,
		Role:           domain.OrganizationRoleMember,
	})
	if err != nil {
		t.Fatalf("CreateOrganizationMembership failed: %v", err)
	}

	return user, personalOrg, personalMembership, teamOrg, teamMembership
}

func assertOrganizationJSON(t *testing.T, got struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}, wantID, wantName, wantRole string) {
	t.Helper()

	if got.ID != wantID || got.Name != wantName || got.Role != wantRole {
		t.Fatalf("expected organization %q %q %q, got %+v", wantID, wantName, wantRole, got)
	}
}

func assertOrganizationListContains(t *testing.T, got []struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}, wantID, wantName, wantRole string) {
	t.Helper()

	for _, org := range got {
		if org.ID == wantID {
			assertOrganizationJSON(t, org, wantID, wantName, wantRole)
			return
		}
	}
	t.Fatalf("expected organization %q in %+v", wantID, got)
}

func withAPIURLParam(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}
