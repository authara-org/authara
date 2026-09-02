package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/oauth/google"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/ratelimiter"
	"github.com/authara-org/authara/internal/session/token"
	"github.com/authara-org/authara/internal/testutil"
)

func TestInvitationPreviewAndPasswordLoginAcceptSwitch(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		h, orgs := newInvitationAPIHandler(t, tdb)
		invited := createPasswordInvitationUser(t, ctx, tdb, "invitation-password@example.com", "password123")
		invite, invitedOrg := createAPIInvitation(t, ctx, tdb, orgs, invited.Email)

		previewResp, err := h.PreviewInvitation(ctx, contract.PreviewInvitationRequestObject{
			Params: contract.PreviewInvitationParams{Token: invite.RawToken},
		})
		if err != nil {
			t.Fatalf("PreviewInvitation failed: %v", err)
		}
		previewRR := httptest.NewRecorder()
		writeContractResponse(t, previewRR, previewResp)
		if previewRR.Code != http.StatusOK {
			t.Fatalf("expected preview status %d, got %d body=%s", http.StatusOK, previewRR.Code, previewRR.Body.String())
		}
		var preview contract.InvitationPreview
		if err := json.Unmarshal(previewRR.Body.Bytes(), &preview); err != nil {
			t.Fatalf("decode preview: %v", err)
		}
		if preview.Organization.Id != invitedOrg.ID || string(preview.Invitation.Email) != invited.Email {
			t.Fatalf("unexpected preview: %+v", preview)
		}

		req := apiJSONRequest(ctx, http.MethodPost, "/auth/api/v1/invitations/login", `{}`)
		resp, err := h.LoginAndAcceptInvitation(contractCtx(ctx, req), contract.LoginAndAcceptInvitationRequestObject{
			Body: &contract.InvitationPasswordLoginRequest{Token: invite.RawToken, Password: "password123"},
		})
		if err != nil {
			t.Fatalf("LoginAndAcceptInvitation failed: %v", err)
		}
		rr := httptest.NewRecorder()
		writeContractResponse(t, rr, resp)
		assertInvitationSession(t, ctx, h, rr, invitedOrg.ID)
		if _, err := tdb.Store.GetOrganizationMembership(ctx, invitedOrg.ID, invited.ID); err != nil {
			t.Fatalf("expected invitation membership: %v", err)
		}
	})
}

func TestAuthenticatedInvitationAcceptSwitchesCurrentSession(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		h, orgs := newInvitationAPIHandler(t, tdb)
		invited := createPasswordInvitationUser(t, ctx, tdb, "invitation-accept@example.com", "password123")
		invite, invitedOrg := createAPIInvitation(t, ctx, tdb, orgs, invited.Email)
		accessToken, _, err := h.Session.CreateSession(ctx, invited.ID, token.AudienceApp, "test", time.Now().UTC())
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}
		identity, err := h.Session.ValidateAccessToken(ctx, accessToken, token.AudienceApp, time.Now().UTC())
		if err != nil {
			t.Fatalf("ValidateAccessToken failed: %v", err)
		}
		authCtx := httpctx.WithSessionID(httpctx.WithUserID(ctx, invited.ID), identity.SessionID)

		resp, err := h.AcceptInvitation(authCtx, contract.AcceptInvitationRequestObject{
			Body: &contract.InvitationTokenRequest{Token: invite.RawToken},
		})
		if err != nil {
			t.Fatalf("AcceptInvitation failed: %v", err)
		}
		rr := httptest.NewRecorder()
		writeContractResponse(t, rr, resp)
		assertInvitationTokens(t, ctx, h, rr, invitedOrg.ID)
	})
}

func TestGoogleInvitationSignupAcceptsAndSwitches(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		h, orgs := newInvitationAPIHandler(t, tdb)
		email := "invitation-google-signup@example.com"
		invite, invitedOrg := createAPIInvitation(t, ctx, tdb, orgs, email)
		h.Google = fakeGoogleVerifier{identity: &google.Identity{OAuthID: "google-invitation-signup", Email: email, EmailVerified: true}}

		req := googleAPIRequest(ctx, "/auth/api/v1/invitations/google", "signup-nonce")
		resp, err := h.AuthenticateAndAcceptInvitationWithGoogle(contractCtx(ctx, req), contract.AuthenticateAndAcceptInvitationWithGoogleRequestObject{
			Body: &contract.InvitationGoogleRequest{
				Token: invite.RawToken, Credential: "credential", Nonce: "signup-nonce", Flow: contract.Signup,
			},
		})
		if err != nil {
			t.Fatalf("AuthenticateAndAcceptInvitationWithGoogle failed: %v", err)
		}
		rr := httptest.NewRecorder()
		writeContractResponse(t, rr, resp)
		assertGoogleInvitationSession(t, ctx, h, rr, invitedOrg.ID)
		user, err := tdb.Store.GetUserByEmail(ctx, email)
		if err != nil {
			t.Fatalf("expected Google user: %v", err)
		}
		if _, err := tdb.Store.GetOrganizationMembership(ctx, invitedOrg.ID, user.ID); err != nil {
			t.Fatalf("expected invitation membership: %v", err)
		}
	})
}

func TestGoogleInvitationCollisionPasswordProofLinksAcceptsAndSwitches(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		h, orgs := newInvitationAPIHandler(t, tdb)
		invited := createPasswordInvitationUser(t, ctx, tdb, "invitation-collision@example.com", "password123")
		invite, invitedOrg := createAPIInvitation(t, ctx, tdb, orgs, invited.Email)
		h.Google = fakeGoogleVerifier{identity: &google.Identity{OAuthID: "google-invitation-collision", Email: invited.Email, EmailVerified: true}}

		googleReq := googleAPIRequest(ctx, "/auth/api/v1/invitations/google", "collision-nonce")
		googleResp, err := h.AuthenticateAndAcceptInvitationWithGoogle(contractCtx(ctx, googleReq), contract.AuthenticateAndAcceptInvitationWithGoogleRequestObject{
			Body: &contract.InvitationGoogleRequest{
				Token: invite.RawToken, Credential: "credential", Nonce: "collision-nonce", Flow: contract.Login,
			},
		})
		if err != nil {
			t.Fatalf("AuthenticateAndAcceptInvitationWithGoogle failed: %v", err)
		}
		googleRR := httptest.NewRecorder()
		writeContractResponse(t, googleRR, googleResp)
		if googleRR.Code != http.StatusOK {
			t.Fatalf("expected recovery status %d, got %d body=%s", http.StatusOK, googleRR.Code, googleRR.Body.String())
		}
		var googleResult contract.InvitationGoogleResult
		if err := json.Unmarshal(googleRR.Body.Bytes(), &googleResult); err != nil {
			t.Fatalf("decode recovery link: %v", err)
		}
		if googleResult.Status != contract.ProofRequired || googleResult.Recovery == nil {
			t.Fatalf("expected proof-required recovery result, got %+v", googleResult)
		}
		link := *googleResult.Recovery
		if len(link.ProofMethods) != 1 || link.ProofMethods[0] != contract.AccountRecoveryLinkProofMethodsPassword {
			t.Fatalf("unexpected proof methods: %+v", link.ProofMethods)
		}

		completeReq := apiJSONRequest(ctx, http.MethodPost, "/auth/api/v1/provider-links/recovery/"+link.LinkId.String()+"/password", `{}`)
		completeResp, err := h.CompleteAccountRecoveryLinkWithPassword(contractCtx(ctx, completeReq), contract.CompleteAccountRecoveryLinkWithPasswordRequestObject{
			LinkID: link.LinkId,
			Body: &contract.AccountRecoveryPasswordProofRequest{
				Password: "password123", InvitationToken: &invite.RawToken,
			},
		})
		if err != nil {
			t.Fatalf("CompleteAccountRecoveryLinkWithPassword failed: %v", err)
		}
		completeRR := httptest.NewRecorder()
		writeContractResponse(t, completeRR, completeResp)
		assertInvitationSession(t, ctx, h, completeRR, invitedOrg.ID)
		if _, err := tdb.Store.GetAuthProviderByProviderAndProviderUserID(ctx, domain.ProviderGoogle, "google-invitation-collision"); err != nil {
			t.Fatalf("expected linked Google provider: %v", err)
		}
	})
}

func TestGoogleInvitationCollisionGoogleProofReplacesIdentityAcceptsAndSwitches(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		h, orgs := newInvitationAPIHandler(t, tdb)
		invited := createGoogleInvitationUser(t, ctx, tdb, "invitation-provider-proof@example.com", "google-existing-proof")
		invite, invitedOrg := createAPIInvitation(t, ctx, tdb, orgs, invited.Email)
		h.Google = fakeGoogleVerifier{identity: &google.Identity{OAuthID: "google-new-collision", Email: invited.Email, EmailVerified: true}}

		googleReq := googleAPIRequest(ctx, "/auth/api/v1/invitations/google", "provider-collision-nonce")
		googleResp, err := h.AuthenticateAndAcceptInvitationWithGoogle(contractCtx(ctx, googleReq), contract.AuthenticateAndAcceptInvitationWithGoogleRequestObject{
			Body: &contract.InvitationGoogleRequest{
				Token: invite.RawToken, Credential: "new-google", Nonce: "provider-collision-nonce", Flow: contract.Login,
			},
		})
		if err != nil {
			t.Fatalf("AuthenticateAndAcceptInvitationWithGoogle failed: %v", err)
		}
		googleRR := httptest.NewRecorder()
		writeContractResponse(t, googleRR, googleResp)
		var googleResult contract.InvitationGoogleResult
		if err := json.Unmarshal(googleRR.Body.Bytes(), &googleResult); err != nil {
			t.Fatalf("decode recovery link: %v", err)
		}
		if googleResult.Recovery == nil || len(googleResult.Recovery.ProofMethods) != 1 || googleResult.Recovery.ProofMethods[0] != contract.AccountRecoveryLinkProofMethodsGoogle {
			t.Fatalf("expected Google proof method, got %+v", googleResult)
		}

		h.Google = fakeGoogleVerifier{identity: &google.Identity{OAuthID: "google-existing-proof", Email: invited.Email, EmailVerified: true}}
		link := googleResult.Recovery
		completeReq := googleAPIRequest(ctx, "/auth/api/v1/provider-links/recovery/"+link.LinkId.String()+"/google", "provider-proof-nonce")
		completeResp, err := h.CompleteAccountRecoveryLinkWithGoogle(contractCtx(ctx, completeReq), contract.CompleteAccountRecoveryLinkWithGoogleRequestObject{
			LinkID: link.LinkId,
			Body: &contract.AccountRecoveryGoogleProofRequest{
				Credential: "existing-google", Nonce: "provider-proof-nonce", InvitationToken: &invite.RawToken,
			},
		})
		if err != nil {
			t.Fatalf("CompleteAccountRecoveryLinkWithGoogle failed: %v", err)
		}
		completeRR := httptest.NewRecorder()
		writeContractResponse(t, completeRR, completeResp)
		assertInvitationSession(t, ctx, h, completeRR, invitedOrg.ID)
		provider, err := tdb.Store.GetAuthProviderByProviderAndProviderUserID(ctx, domain.ProviderGoogle, "google-new-collision")
		if err != nil {
			t.Fatalf("expected replacement Google provider identity: %v", err)
		}
		if provider.UserID != invited.ID {
			t.Fatalf("expected provider linked to %s, got %s", invited.ID, provider.UserID)
		}
	})
}

type fakeGoogleVerifier struct {
	identity *google.Identity
}

func (f fakeGoogleVerifier) VerifyIDToken(_ context.Context, _ string, _ string) (*google.Identity, error) {
	return f.identity, nil
}

func newInvitationAPIHandler(t *testing.T, tdb *testutil.TestDB) (*APIHandler, *organization.Service) {
	t.Helper()
	providers := googleTestProviders()
	orgs := organization.New(organization.Config{
		Store:         tdb.Store,
		Tx:            tdb.Tx,
		Mode:          organization.OrgModeMulti,
		InvitationTTL: time.Hour,
	})
	return &APIHandler{
		Auth: auth.New(auth.Config{
			Store: tdb.Store, Tx: tdb.Tx, Organizations: orgs, OAuthProviders: providers,
		}),
		Session:        newAPIHandlerTestSessionService(t, tdb),
		Organizations:  orgs,
		Limiter:        ratelimiter.NewInMemoryLimiter(ratelimiter.LimiterConfig{}),
		OAuthProviders: providers,
		AccessTTL:      time.Minute,
		RefreshTTL:     time.Hour,
	}, orgs
}

func createPasswordInvitationUser(t *testing.T, ctx context.Context, tdb *testutil.TestDB, email string, password string) domain.User {
	t.Helper()
	user, err := tdb.Store.CreateUser(ctx, domain.User{Email: email, Username: email[:len(email)-len("@example.com")]})
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	if _, _, err := tdb.Store.EnsureOrganizationForUser(ctx, user.ID, user.Username, domain.OrganizationKindTeam); err != nil {
		t.Fatalf("EnsureOrganizationForUser failed: %v", err)
	}
	hash, err := auth.Hash(password)
	if err != nil {
		t.Fatalf("Hash failed: %v", err)
	}
	if _, err := tdb.Store.CreateAuthProvider(ctx, domain.AuthProvider{
		UserID: user.ID, Provider: domain.ProviderPassword, PasswordHash: &hash,
	}); err != nil {
		t.Fatalf("CreateAuthProvider failed: %v", err)
	}
	return user
}

func createGoogleInvitationUser(t *testing.T, ctx context.Context, tdb *testutil.TestDB, email string, oauthID string) domain.User {
	t.Helper()
	user, err := tdb.Store.CreateUser(ctx, domain.User{Email: email, Username: email[:len(email)-len("@example.com")]})
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	if _, _, err := tdb.Store.EnsureOrganizationForUser(ctx, user.ID, user.Username, domain.OrganizationKindTeam); err != nil {
		t.Fatalf("EnsureOrganizationForUser failed: %v", err)
	}
	if _, err := tdb.Store.CreateAuthProvider(ctx, domain.AuthProvider{
		UserID: user.ID, Provider: domain.ProviderGoogle, ProviderUserID: &oauthID,
	}); err != nil {
		t.Fatalf("CreateAuthProvider failed: %v", err)
	}
	return user
}

func createAPIInvitation(t *testing.T, ctx context.Context, tdb *testutil.TestDB, orgs *organization.Service, email string) (organization.InvitationWithToken, domain.Organization) {
	t.Helper()
	owner, err := tdb.Store.CreateUser(ctx, domain.User{Email: "owner-" + email, Username: "owner-" + email[:len(email)-len("@example.com")]})
	if err != nil {
		t.Fatalf("CreateUser owner failed: %v", err)
	}
	org, _, err := tdb.Store.EnsureOrganizationForUser(ctx, owner.ID, owner.Username, domain.OrganizationKindTeam)
	if err != nil {
		t.Fatalf("EnsureOrganizationForUser owner failed: %v", err)
	}
	invite, err := orgs.CreateInvitation(ctx, organization.CreateInvitationInput{
		OrganizationID: org.ID,
		ActorUserID:    owner.ID,
		Email:          email,
		Now:            time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("CreateInvitation failed: %v", err)
	}
	return invite, org
}

func googleAPIRequest(ctx context.Context, target string, nonce string) *http.Request {
	req := apiJSONRequest(ctx, http.MethodPost, target, `{}`)
	req.AddCookie(&http.Cookie{Name: "authara_oauth_nonce", Value: nonce})
	return req
}

func assertInvitationSession(t *testing.T, ctx context.Context, h *APIHandler, rr *httptest.ResponseRecorder, organizationID contract.OrganizationID) {
	t.Helper()
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}
	var body contract.AuthSession
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	assertAccessOrganization(t, ctx, h, body.AccessToken, organizationID)
}

func assertGoogleInvitationSession(t *testing.T, ctx context.Context, h *APIHandler, rr *httptest.ResponseRecorder, organizationID contract.OrganizationID) {
	t.Helper()
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}
	var body contract.InvitationGoogleResult
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode Google invitation result: %v", err)
	}
	if body.Status != contract.Authenticated || body.Session == nil {
		t.Fatalf("expected authenticated Google invitation result, got %+v", body)
	}
	assertAccessOrganization(t, ctx, h, body.Session.AccessToken, organizationID)
}

func assertInvitationTokens(t *testing.T, ctx context.Context, h *APIHandler, rr *httptest.ResponseRecorder, organizationID contract.OrganizationID) {
	t.Helper()
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}
	var body contract.Tokens
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode tokens: %v", err)
	}
	assertAccessOrganization(t, ctx, h, body.AccessToken, organizationID)
}

func assertAccessOrganization(t *testing.T, ctx context.Context, h *APIHandler, accessToken string, organizationID contract.OrganizationID) {
	t.Helper()
	identity, err := h.Session.ValidateAccessToken(ctx, accessToken, token.AudienceApp, time.Now().UTC())
	if err != nil {
		t.Fatalf("ValidateAccessToken failed: %v", err)
	}
	if identity.OrganizationID != organizationID {
		t.Fatalf("expected organization %s, got %s", organizationID, identity.OrganizationID)
	}
}

var _ GoogleVerifier = fakeGoogleVerifier{}
