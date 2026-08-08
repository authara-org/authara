package ui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/accesspolicy"
	authsvc "github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/render"
	"github.com/authara-org/authara/internal/oauth"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/ratelimiter"
	"github.com/authara-org/authara/internal/testutil"
)

func TestSingleModeInvitationWaitsForExistingOrganizationDeparture(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		organizations := organization.New(organization.Config{
			Store:         tdb.Store,
			Tx:            tdb.Tx,
			Mode:          organization.OrgModeSingle,
			InvitationTTL: time.Hour,
		})
		targetOwner, err := tdb.Store.CreateUser(ctx, domain.User{Email: "single-target-owner@example.com", Username: "single-target-owner"})
		if err != nil {
			t.Fatal(err)
		}
		targetOrg, _, err := tdb.Store.EnsureOrganizationForUser(ctx, targetOwner.ID, "Target", domain.OrganizationKindTeam)
		if err != nil {
			t.Fatal(err)
		}
		invitee, err := tdb.Store.CreateUser(ctx, domain.User{Email: "single-existing@example.com", Username: "single-existing"})
		if err != nil {
			t.Fatal(err)
		}
		oldOrg, _, err := tdb.Store.EnsureOrganizationForUser(ctx, invitee.ID, "Old", domain.OrganizationKindTeam)
		if err != nil {
			t.Fatal(err)
		}
		invite, err := organizations.CreateInvitation(ctx, organization.CreateInvitationInput{
			OrganizationID: targetOrg.ID,
			ActorUserID:    targetOwner.ID,
			Email:          invitee.Email,
			Now:            time.Now().UTC(),
		})
		if err != nil {
			t.Fatal(err)
		}
		h := &UIHandler{
			Auth:          authsvc.New(authsvc.Config{Store: tdb.Store, Tx: tdb.Tx, Organizations: organizations}),
			Organizations: organizations,
			Render:        render.New(render.Assets{}, false),
		}

		signedInCtx := httpctx.WithUserID(ctx, invitee.ID)
		req := httptest.NewRequest(http.MethodGet, "/auth/invitations/accept?token="+url.QueryEscape(invite.RawToken), nil).WithContext(signedInCtx)
		rr := httptest.NewRecorder()
		h.InvitationAcceptPage(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
		if body := rr.Body.String(); !strings.Contains(body, "Leave or delete your current organization through the application") || strings.Contains(body, `action="/auth/invitations/accept"`) {
			t.Fatalf("expected blocking departure prompt, body=%s", body)
		}

		if err := organizations.DeleteOrganization(ctx, organization.DeleteOrganizationInput{OrganizationID: oldOrg.ID, ActorUserID: invitee.ID}); err != nil {
			t.Fatalf("DeleteOrganization failed: %v", err)
		}
		req = httptest.NewRequest(http.MethodGet, "/auth/invitations/accept?token="+url.QueryEscape(invite.RawToken), nil).WithContext(httpctx.WithUserID(ctx, invitee.ID))
		rr = httptest.NewRecorder()
		h.InvitationAcceptPage(rr, req)
		if body := rr.Body.String(); !strings.Contains(body, `action="/auth/invitations/accept"`) || !strings.Contains(body, "Join Target") {
			t.Fatalf("expected invitation acceptance after departure, body=%s", body)
		}
	})
}

func TestInvitationPasswordLoginUsesValidTokenForAdmission(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		h, invite, email, _ := newInvitationLoginTestHandler(t, ctx, tdb, "password", domain.ProviderPassword)

		invalid := invitationLoginRequest(ctx, "not-a-real-token", "correct-password")
		invalidRR := httptest.NewRecorder()
		h.InvitationLoginPost(invalidRR, invalid)
		if invalidRR.Code != http.StatusBadRequest {
			t.Fatalf("expected invalid invitation status %d, got %d", http.StatusBadRequest, invalidRR.Code)
		}
		assertEmailAllowed(t, ctx, tdb, email, false)

		valid := invitationLoginRequest(ctx, invite.RawToken, "wrong-password")
		validRR := httptest.NewRecorder()
		h.InvitationLoginPost(validRR, valid)
		if validRR.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected invalid credentials status %d, got %d", http.StatusUnprocessableEntity, validRR.Code)
		}
		assertEmailAllowed(t, ctx, tdb, email, true)
	})
}

func TestInvitationOAuthLoginUsesValidTokenForAdmission(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		h, invite, email, oauthID := newInvitationLoginTestHandler(t, ctx, tdb, "oauth", domain.ProviderGoogle)

		req := httptest.NewRequest(http.MethodPost, "/auth/oauth/google/callback", nil).WithContext(ctx)
		rr := httptest.NewRecorder()
		returnTo := invitationAuthURL("/auth/invitations/login", invite.RawToken)
		h.finishInvitationOAuth(rr, req, "/auth/invitations/login", invite.RawToken, returnTo, email, true, oauthID)

		if location := rr.Header().Get("X-Authara-Redirect"); !strings.HasPrefix(location, "/auth/provider-links/confirm?") {
			t.Fatalf("expected provider-link redirect, got %q", location)
		}
		assertEmailAllowed(t, ctx, tdb, email, true)
	})
}

func newInvitationLoginTestHandler(
	t *testing.T,
	ctx context.Context,
	tdb *testutil.TestDB,
	suffix string,
	provider domain.Provider,
) (*UIHandler, organization.InvitationWithToken, string, string) {
	t.Helper()

	owner, err := tdb.Store.CreateUser(ctx, domain.User{
		Email:    "invite-owner-" + suffix + "@example.com",
		Username: "invite-owner-" + suffix,
	})
	if err != nil {
		t.Fatalf("CreateUser owner failed: %v", err)
	}
	org, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, owner.ID, owner.Username)
	if err != nil {
		t.Fatalf("EnsureDefaultOrganizationForUser failed: %v", err)
	}

	email := "invite-existing-" + suffix + "@example.com"
	user, err := tdb.Store.CreateUser(ctx, domain.User{
		Email:    email,
		Username: "invite-existing-" + suffix,
	})
	if err != nil {
		t.Fatalf("CreateUser invitee failed: %v", err)
	}

	oauthID := ""
	authProvider := domain.AuthProvider{UserID: user.ID, Provider: provider}
	switch provider {
	case domain.ProviderPassword:
		passwordHash, err := authsvc.Hash("correct-password")
		if err != nil {
			t.Fatalf("Hash failed: %v", err)
		}
		authProvider.PasswordHash = &passwordHash
	case domain.ProviderGoogle:
		oauthID = "google-invite-existing-" + suffix
		// Leave Google unlinked so the callback reaches the provider-link flow.
	default:
		t.Fatalf("unsupported test provider %q", provider)
	}
	if provider == domain.ProviderPassword {
		if _, err := tdb.Store.CreateAuthProvider(ctx, authProvider); err != nil {
			t.Fatalf("CreateAuthProvider failed: %v", err)
		}
	}

	organizations := organization.New(organization.Config{
		Store:         tdb.Store,
		Tx:            tdb.Tx,
		Mode:          organization.OrgModeMulti,
		InvitationTTL: time.Hour,
	})
	invite, err := organizations.CreateInvitation(ctx, organization.CreateInvitationInput{
		OrganizationID: org.ID,
		ActorUserID:    owner.ID,
		Email:          email,
		Now:            time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("CreateInvitation failed: %v", err)
	}

	policy := accesspolicy.New(accesspolicy.Config{Store: tdb.Store, Enabled: true})
	providers := oauth.OAuthProviders{Providers: []oauth.OAuthProvider{
		oauth.NewOAuthProvider(domain.ProviderGoogle, "test-google-client-id", "http://localhost:3000"),
	}}
	return &UIHandler{
		Auth: authsvc.New(authsvc.Config{
			Store:          tdb.Store,
			Tx:             tdb.Tx,
			AccessPolicy:   policy,
			Organizations:  organizations,
			OAuthProviders: providers,
		}),
		Organizations:  organizations,
		Limiter:        ratelimiter.NewInMemoryLimiter(ratelimiter.LimiterConfig{}),
		OAuthProviders: providers,
		Render:         render.New(render.Assets{}, false),
	}, invite, email, oauthID
}

func invitationLoginRequest(ctx context.Context, token string, password string) *http.Request {
	form := url.Values{}
	form.Set("token", token)
	form.Set("password", password)
	req := httptest.NewRequest(http.MethodPost, "/auth/invitations/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req.WithContext(ctx)
}

func assertEmailAllowed(t *testing.T, ctx context.Context, tdb *testutil.TestDB, email string, want bool) {
	t.Helper()
	allowed, err := tdb.Store.IsEmailAllowed(ctx, email)
	if err != nil {
		t.Fatalf("IsEmailAllowed failed: %v", err)
	}
	if allowed != want {
		t.Fatalf("IsEmailAllowed(%q) = %t, want %t", email, allowed, want)
	}
}
