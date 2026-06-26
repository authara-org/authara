package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/accesspolicy"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/oauth"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/session/roles"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/google/uuid"
)

type staticAccessPolicy struct {
	allowed bool
	err     error
}

func (p staticAccessPolicy) IsEmailAllowed(ctx context.Context, email string) (bool, error) {
	if p.err != nil {
		return false, p.err
	}
	return p.allowed, nil
}

func testOrganizations(tdb *testutil.TestDB) *organization.Service {
	return organization.New(organization.Config{Store: tdb.Store, Tx: tdb.Tx, Mode: organization.OrgModeSingle})
}

func TestNew_DefaultsNilDependencies(t *testing.T) {
	svc := New(Config{})

	if svc.webhookPublisher == nil {
		t.Fatal("expected default webhook publisher to be set")
	}
	if svc.accessPolicy == nil {
		t.Fatal("expected default access policy to be set")
	}
	if svc.organizations != nil {
		t.Fatal("expected organization service to require explicit configuration")
	}
}

func TestGetUser(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		created, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "get-user@example.com",
			Username: "get-user",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}

		svc := New(Config{
			Store: tdb.Store,
			Tx:    tdb.Tx,
		})

		got, err := svc.GetUser(ctx, created.ID)
		if err != nil {
			t.Fatalf("GetUser failed: %v", err)
		}
		if got.ID != created.ID {
			t.Fatalf("expected user id %q, got %q", created.ID, got.ID)
		}
	})
}

func TestUserExistsByEmail(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		svc := New(Config{
			Store: tdb.Store,
			Tx:    tdb.Tx,
		})

		exists, err := svc.UserExistsByEmail(ctx, "missing@example.com")
		if err != nil {
			t.Fatalf("UserExistsByEmail returned error: %v", err)
		}
		if exists {
			t.Fatal("expected exists=false for missing user")
		}

		_, err = tdb.Store.CreateUser(ctx, domain.User{
			Email:    "exists@example.com",
			Username: "exists-user",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}

		exists, err = svc.UserExistsByEmail(ctx, "exists@example.com")
		if err != nil {
			t.Fatalf("UserExistsByEmail returned error: %v", err)
		}
		if !exists {
			t.Fatal("expected exists=true for existing user")
		}
	})
}

func TestSignup_WithPassword_Succeeds(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		svc := New(Config{
			Store:         tdb.Store,
			Tx:            tdb.Tx,
			AccessPolicy:  staticAccessPolicy{allowed: true},
			Organizations: testOrganizations(tdb),
		})

		user, err := svc.Signup(ctx, SignupInput{
			Provider:     domain.ProviderPassword,
			Email:        "signup@example.com",
			Username:     "signup-user",
			PasswordHash: "hashed-password",
		})
		if err != nil {
			t.Fatalf("Signup failed: %v", err)
		}
		if user.Email != "signup@example.com" {
			t.Fatalf("expected email signup@example.com, got %q", user.Email)
		}
		if user.Username != "signup-user" {
			t.Fatalf("expected username signup-user, got %q", user.Username)
		}
		org, membership := userOnlyOrganization(t, ctx, tdb, user.ID)
		if org.Kind != domain.OrganizationKindTeam || membership.Role != domain.OrganizationRoleOwner {
			t.Fatalf("expected team owner org, got org=%+v membership=%+v", org, membership)
		}
	})
}

func TestSignup_WithPassword_UsesOrganizationMode(t *testing.T) {
	tests := []struct {
		mode     organization.OrgMode
		wantKind domain.OrganizationKind
	}{
		{organization.OrgModePersonal, domain.OrganizationKindPersonal},
		{organization.OrgModeSingle, domain.OrganizationKindTeam},
		{organization.OrgModeMulti, domain.OrganizationKindPersonal},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			tdb := testutil.OpenTestDB(t)

			testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
				orgs := organization.New(organization.Config{
					Store: tdb.Store,
					Tx:    tdb.Tx,
					Mode:  tt.mode,
				})
				svc := New(Config{
					Store:         tdb.Store,
					Tx:            tdb.Tx,
					AccessPolicy:  staticAccessPolicy{allowed: true},
					Organizations: orgs,
				})

				user, err := svc.Signup(ctx, SignupInput{
					Provider:     domain.ProviderPassword,
					Email:        "signup-" + string(tt.mode) + "@example.com",
					Username:     "signup-" + string(tt.mode),
					PasswordHash: "hashed-password",
				})
				if err != nil {
					t.Fatalf("Signup failed: %v", err)
				}

				org, membership := userOnlyOrganization(t, ctx, tdb, user.ID)
				if org.Kind != tt.wantKind || membership.Role != domain.OrganizationRoleOwner {
					t.Fatalf("expected %s owner org, got org=%+v membership=%+v", tt.wantKind, org, membership)
				}
			})
		})
	}
}

func userOnlyOrganization(t *testing.T, ctx context.Context, tdb *testutil.TestDB, userID uuid.UUID) (domain.Organization, domain.OrganizationMembership) {
	t.Helper()

	memberships, err := tdb.Store.ListOrganizationMembershipsByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("ListOrganizationMembershipsByUserID failed: %v", err)
	}
	if len(memberships) != 1 {
		t.Fatalf("expected 1 membership, got %d", len(memberships))
	}
	org, err := tdb.Store.GetOrganizationByID(ctx, memberships[0].OrganizationID)
	if err != nil {
		t.Fatalf("GetOrganizationByID failed: %v", err)
	}
	return org, memberships[0]
}

func TestSignup_WithInvitationSkipsDefaultOrganization(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		owner, err := tdb.Store.CreateUser(ctx, domain.User{Email: "invite-owner@example.com", Username: "invite-owner"})
		if err != nil {
			t.Fatalf("CreateUser owner failed: %v", err)
		}
		org, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, owner.ID, owner.Username)
		if err != nil {
			t.Fatalf("EnsureDefaultOrganizationForUser failed: %v", err)
		}

		orgs := organization.New(organization.Config{
			Store:         tdb.Store,
			Tx:            tdb.Tx,
			Mode:          organization.OrgModeSingle,
			InvitationTTL: time.Hour,
		})
		invite, err := orgs.CreateInvitation(ctx, organization.CreateInvitationInput{
			OrganizationID: org.ID,
			ActorUserID:    owner.ID,
			Email:          "invited-signup@example.com",
			Now:            time.Now().UTC(),
		})
		if err != nil {
			t.Fatalf("CreateInvitation failed: %v", err)
		}

		svc := New(Config{
			Store:         tdb.Store,
			Tx:            tdb.Tx,
			AccessPolicy:  staticAccessPolicy{allowed: true},
			Organizations: orgs,
		})

		user, err := svc.Signup(ctx, SignupInput{
			Provider:        domain.ProviderPassword,
			Email:           "invited-signup@example.com",
			Username:        "invited-signup",
			PasswordHash:    "hashed-password",
			InvitationToken: invite.RawToken,
		})
		if err != nil {
			t.Fatalf("Signup failed: %v", err)
		}

		_, _, err = tdb.Store.GetPersonalOrganizationForUser(ctx, user.ID)
		if !errors.Is(err, store.ErrOrganizationNotFound) {
			t.Fatalf("expected no personal default org, got %v", err)
		}
		membership, err := tdb.Store.GetOrganizationMembership(ctx, org.ID, user.ID)
		if err != nil {
			t.Fatalf("GetOrganizationMembership failed: %v", err)
		}
		if membership.Role != domain.OrganizationRoleMember {
			t.Fatalf("expected member role, got %q", membership.Role)
		}
	})
}

func TestSignup_WithInvitationInMultiCreatesPersonalAndJoinsInvite(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		owner, err := tdb.Store.CreateUser(ctx, domain.User{Email: "multi-owner@example.com", Username: "multi-owner"})
		if err != nil {
			t.Fatalf("CreateUser owner failed: %v", err)
		}
		org, _, err := tdb.Store.EnsureOrganizationForUser(ctx, owner.ID, owner.Username, domain.OrganizationKindTeam)
		if err != nil {
			t.Fatalf("EnsureOrganizationForUser owner failed: %v", err)
		}

		orgs := organization.New(organization.Config{
			Store:         tdb.Store,
			Tx:            tdb.Tx,
			Mode:          organization.OrgModeMulti,
			InvitationTTL: time.Hour,
		})
		invite, err := orgs.CreateInvitation(ctx, organization.CreateInvitationInput{
			OrganizationID: org.ID,
			ActorUserID:    owner.ID,
			Email:          "multi-invited@example.com",
			Now:            time.Now().UTC(),
		})
		if err != nil {
			t.Fatalf("CreateInvitation failed: %v", err)
		}

		svc := New(Config{
			Store:         tdb.Store,
			Tx:            tdb.Tx,
			AccessPolicy:  staticAccessPolicy{allowed: true},
			Organizations: orgs,
		})

		user, err := svc.Signup(ctx, SignupInput{
			Provider:        domain.ProviderPassword,
			Email:           "multi-invited@example.com",
			Username:        "multi-invited",
			PasswordHash:    "hashed-password",
			InvitationToken: invite.RawToken,
		})
		if err != nil {
			t.Fatalf("Signup failed: %v", err)
		}

		personal, ownerMembership, err := tdb.Store.GetPersonalOrganizationForUser(ctx, user.ID)
		if err != nil {
			t.Fatalf("GetPersonalOrganizationForUser failed: %v", err)
		}
		if personal.Kind != domain.OrganizationKindPersonal || ownerMembership.Role != domain.OrganizationRoleOwner {
			t.Fatalf("expected personal owner org, got org=%+v membership=%+v", personal, ownerMembership)
		}
		invitedMembership, err := tdb.Store.GetOrganizationMembership(ctx, org.ID, user.ID)
		if err != nil {
			t.Fatalf("GetOrganizationMembership failed: %v", err)
		}
		if invitedMembership.Role != domain.OrganizationRoleMember {
			t.Fatalf("expected invited member role, got %q", invitedMembership.Role)
		}
	})
}

func TestSignup_WithInvitationAddsEmailToAllowlist(t *testing.T) {
	tests := []struct {
		name  string
		useID bool
	}{
		{name: "token", useID: false},
		{name: "id", useID: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tdb := testutil.OpenTestDB(t)

			testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
				owner, err := tdb.Store.CreateUser(ctx, domain.User{
					Email:    "allowlist-invite-owner-" + tt.name + "@example.com",
					Username: "allowlist-invite-owner-" + tt.name,
				})
				if err != nil {
					t.Fatalf("CreateUser owner failed: %v", err)
				}
				org, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, owner.ID, owner.Username)
				if err != nil {
					t.Fatalf("EnsureDefaultOrganizationForUser failed: %v", err)
				}

				orgs := organization.New(organization.Config{
					Store:         tdb.Store,
					Tx:            tdb.Tx,
					Mode:          organization.OrgModeSingle,
					InvitationTTL: time.Hour,
				})
				invitedEmail := "allowlist-invited-" + tt.name + "@example.com"
				invite, err := orgs.CreateInvitation(ctx, organization.CreateInvitationInput{
					OrganizationID: org.ID,
					ActorUserID:    owner.ID,
					Email:          invitedEmail,
					Now:            time.Now().UTC(),
				})
				if err != nil {
					t.Fatalf("CreateInvitation failed: %v", err)
				}

				input := SignupInput{
					Provider:     domain.ProviderPassword,
					Email:        invitedEmail,
					Username:     "allowlist-invited-" + tt.name,
					PasswordHash: "hashed-password",
				}
				if tt.useID {
					input.InvitationID = invite.Invitation.ID
				} else {
					input.InvitationToken = invite.RawToken
				}

				svc := New(Config{
					Store:         tdb.Store,
					Tx:            tdb.Tx,
					AccessPolicy:  accesspolicy.New(accesspolicy.Config{Store: tdb.Store, Enabled: true}),
					Organizations: orgs,
				})

				user, err := svc.Signup(ctx, input)
				if err != nil {
					t.Fatalf("Signup failed: %v", err)
				}

				allowed, err := tdb.Store.IsEmailAllowed(ctx, invitedEmail)
				if err != nil {
					t.Fatalf("IsEmailAllowed failed: %v", err)
				}
				if !allowed {
					t.Fatal("expected invited email to be allowlisted")
				}

				if _, err := tdb.Store.GetOrganizationMembership(ctx, org.ID, user.ID); err != nil {
					t.Fatalf("expected invitation membership to be created: %v", err)
				}
			})
		})
	}
}

func TestSignup_WithInvalidInvitationDoesNotAddEmailToAllowlist(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		orgs := organization.New(organization.Config{
			Store:         tdb.Store,
			Tx:            tdb.Tx,
			Mode:          organization.OrgModeSingle,
			InvitationTTL: time.Hour,
		})
		svc := New(Config{
			Store:         tdb.Store,
			Tx:            tdb.Tx,
			AccessPolicy:  accesspolicy.New(accesspolicy.Config{Store: tdb.Store, Enabled: true}),
			Organizations: orgs,
		})

		email := "invalid-invite-allowlist@example.com"
		_, err := svc.Signup(ctx, SignupInput{
			Provider:        domain.ProviderPassword,
			Email:           email,
			Username:        "invalid-invite-allowlist",
			PasswordHash:    "hashed-password",
			InvitationToken: "not-a-real-token",
		})
		if !errors.Is(err, store.ErrOrganizationInvitationNotFound) {
			t.Fatalf("expected ErrOrganizationInvitationNotFound, got %v", err)
		}

		allowed, err := tdb.Store.IsEmailAllowed(ctx, email)
		if err != nil {
			t.Fatalf("IsEmailAllowed failed: %v", err)
		}
		if allowed {
			t.Fatal("expected invalid invitation email not to be allowlisted")
		}
	})
}

func TestSignup_GeneratesUsernameWhenEmpty(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		svc := New(Config{
			Store:         tdb.Store,
			Tx:            tdb.Tx,
			AccessPolicy:  staticAccessPolicy{allowed: true},
			Organizations: testOrganizations(tdb),
		})

		user, err := svc.Signup(ctx, SignupInput{
			Provider:     domain.ProviderPassword,
			Email:        "john.doe@example.com",
			Username:     "",
			PasswordHash: "hashed-password",
		})
		if err != nil {
			t.Fatalf("Signup failed: %v", err)
		}
		if user.Username == "" {
			t.Fatal("expected generated username, got empty string")
		}
	})
}

func TestSignup_BlockedByAccessPolicy(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		svc := New(Config{
			Store:        tdb.Store,
			Tx:           tdb.Tx,
			AccessPolicy: staticAccessPolicy{allowed: false},
		})

		_, err := svc.Signup(ctx, SignupInput{
			Provider:     domain.ProviderPassword,
			Email:        "blocked@example.com",
			Username:     "blocked-user",
			PasswordHash: "hashed-password",
		})
		if !errors.Is(err, ErrEmailNotAllowed) {
			t.Fatalf("expected ErrEmailNotAllowed, got %v", err)
		}
	})
}

func TestSignup_UnsupportedProvider(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		svc := New(Config{
			Store:        tdb.Store,
			Tx:           tdb.Tx,
			AccessPolicy: staticAccessPolicy{allowed: true},
			OAuthProviders: oauth.OAuthProviders{
				Providers: []oauth.OAuthProvider{
					oauth.NewOAuthProvider(domain.ProviderGoogle, "test-google-client-id", "http://localhost:3000"),
				},
			},
		})

		_, err := svc.Signup(ctx, SignupInput{
			Provider: domain.ProviderGoogle,
			Email:    "oauth-signup@example.com",
			Username: "oauth-signup",
		})
		if !errors.Is(err, ErrUnsupportedProvider) {
			t.Fatalf("expected ErrUnsupportedProvider, got %v", err)
		}
	})
}

func TestLogin_WithPassword_Succeeds(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		passwordHash, err := Hash("super-secret")
		if err != nil {
			t.Fatalf("Hash failed: %v", err)
		}

		user := createPasswordUser(t, ctx, tdb, "login@example.com", "login-user", passwordHash)

		svc := New(Config{
			Store:        tdb.Store,
			Tx:           tdb.Tx,
			AccessPolicy: staticAccessPolicy{allowed: true},
		})

		got, err := svc.Login(ctx, LoginInput{
			Provider: domain.ProviderPassword,
			Email:    user.Email,
			Password: "super-secret",
		})
		if err != nil {
			t.Fatalf("Login failed: %v", err)
		}
		if got.ID != user.ID {
			t.Fatalf("expected user id %q, got %q", user.ID, got.ID)
		}
	})
}

func TestLogin_WrongPassword(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		passwordHash, err := Hash("correct-password")
		if err != nil {
			t.Fatalf("Hash failed: %v", err)
		}

		_ = createPasswordUser(t, ctx, tdb, "wrong-pass@example.com", "wrong-pass-user", passwordHash)

		svc := New(Config{
			Store:        tdb.Store,
			Tx:           tdb.Tx,
			AccessPolicy: staticAccessPolicy{allowed: true},
		})

		_, err = svc.Login(ctx, LoginInput{
			Provider: domain.ProviderPassword,
			Email:    "wrong-pass@example.com",
			Password: "wrong-password",
		})
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("expected ErrInvalidCredentials, got %v", err)
		}
	})
}

func TestLogin_BlockedByAccessPolicy(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		svc := New(Config{
			Store:        tdb.Store,
			Tx:           tdb.Tx,
			AccessPolicy: staticAccessPolicy{allowed: false},
		})

		_, err := svc.Login(ctx, LoginInput{
			Provider: domain.ProviderPassword,
			Email:    "blocked@example.com",
			Password: "irrelevant",
		})
		if !errors.Is(err, ErrEmailNotAllowed) {
			t.Fatalf("expected ErrEmailNotAllowed, got %v", err)
		}
	})
}

func TestLogin_UnsupportedProvider(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		svc := New(Config{
			Store:        tdb.Store,
			Tx:           tdb.Tx,
			AccessPolicy: staticAccessPolicy{allowed: true},
		})

		_, err := svc.Login(ctx, LoginInput{
			Provider: domain.Provider("unknown"),
			Email:    "user@example.com",
		})
		if !errors.Is(err, ErrUnsupportedProvider) {
			t.Fatalf("expected ErrUnsupportedProvider, got %v", err)
		}
	})
}

func TestLoginWithExternalIdentity_CreatesUserWhenMissing(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		svc := New(Config{
			Store:         tdb.Store,
			Tx:            tdb.Tx,
			AccessPolicy:  staticAccessPolicy{allowed: true},
			Organizations: testOrganizations(tdb),
			OAuthProviders: oauth.OAuthProviders{
				Providers: []oauth.OAuthProvider{
					oauth.NewOAuthProvider(domain.ProviderGoogle, "test-google-client-id", "http://localhost:3000"),
				},
			},
		})

		user, err := svc.Login(ctx, LoginInput{
			Provider: domain.ProviderGoogle,
			Email:    "oauth-new@example.com",
			Username: "oauth-new",
			OAuthID:  "google-oauth-id-123",
		})
		if err != nil {
			t.Fatalf("Login failed: %v", err)
		}

		// Verify user persisted
		dbUser, err := tdb.Store.GetUserByEmail(ctx, "oauth-new@example.com")
		if err != nil {
			t.Fatalf("expected user to be created in DB: %v", err)
		}
		if dbUser.ID != user.ID {
			t.Fatalf("expected DB user id %q, got %q", user.ID, dbUser.ID)
		}

		// Verify provider persisted
		provider, err := tdb.Store.GetAuthProviderByProviderAndProviderUserID(
			ctx,
			domain.ProviderGoogle,
			"google-oauth-id-123",
		)
		if err != nil {
			t.Fatalf("expected auth provider to be created: %v", err)
		}

		if provider.UserID != user.ID {
			t.Fatalf("expected provider user_id %q, got %q", user.ID, provider.UserID)
		}
		org, membership := userOnlyOrganization(t, ctx, tdb, user.ID)
		if org.Kind != domain.OrganizationKindTeam || membership.Role != domain.OrganizationRoleOwner {
			t.Fatalf("expected team owner org, got org=%+v membership=%+v", org, membership)
		}
	})
}

func TestLoginWithExternalIdentity_WithInvitationAddsEmailToAllowlist(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		owner, err := tdb.Store.CreateUser(ctx, domain.User{Email: "oauth-invite-owner@example.com", Username: "oauth-invite-owner"})
		if err != nil {
			t.Fatalf("CreateUser owner failed: %v", err)
		}
		org, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, owner.ID, owner.Username)
		if err != nil {
			t.Fatalf("EnsureDefaultOrganizationForUser failed: %v", err)
		}

		orgs := organization.New(organization.Config{
			Store:         tdb.Store,
			Tx:            tdb.Tx,
			Mode:          organization.OrgModeSingle,
			InvitationTTL: time.Hour,
		})
		invitedEmail := "oauth-invited@example.com"
		invite, err := orgs.CreateInvitation(ctx, organization.CreateInvitationInput{
			OrganizationID: org.ID,
			ActorUserID:    owner.ID,
			Email:          invitedEmail,
			Now:            time.Now().UTC(),
		})
		if err != nil {
			t.Fatalf("CreateInvitation failed: %v", err)
		}

		svc := New(Config{
			Store:         tdb.Store,
			Tx:            tdb.Tx,
			AccessPolicy:  accesspolicy.New(accesspolicy.Config{Store: tdb.Store, Enabled: true}),
			Organizations: orgs,
			OAuthProviders: oauth.OAuthProviders{
				Providers: []oauth.OAuthProvider{
					oauth.NewOAuthProvider(domain.ProviderGoogle, "test-google-client-id", "http://localhost:3000"),
				},
			},
		})

		user, err := svc.Login(ctx, LoginInput{
			Provider:        domain.ProviderGoogle,
			Email:           invitedEmail,
			Username:        "oauth-invited",
			OAuthID:         "google-oauth-invited-id",
			InvitationToken: invite.RawToken,
		})
		if err != nil {
			t.Fatalf("Login failed: %v", err)
		}

		allowed, err := tdb.Store.IsEmailAllowed(ctx, invitedEmail)
		if err != nil {
			t.Fatalf("IsEmailAllowed failed: %v", err)
		}
		if !allowed {
			t.Fatal("expected invited email to be allowlisted")
		}

		if _, err := tdb.Store.GetOrganizationMembership(ctx, org.ID, user.ID); err != nil {
			t.Fatalf("expected invitation membership to be created: %v", err)
		}
	})
}

func TestLoginWithExternalIdentity_ExistingEmailMustLink(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		_, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "oauth-link@example.com",
			Username: "existing-user",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}

		svc := New(Config{
			Store:         tdb.Store,
			Tx:            tdb.Tx,
			AccessPolicy:  staticAccessPolicy{allowed: true},
			Organizations: testOrganizations(tdb),
			OAuthProviders: oauth.OAuthProviders{
				Providers: []oauth.OAuthProvider{
					oauth.NewOAuthProvider(domain.ProviderGoogle, "test-google-client-id", "http://localhost:3000"),
				},
			},
		})

		_, err = svc.Login(ctx, LoginInput{
			Provider: domain.ProviderGoogle,
			Email:    "oauth-link@example.com",
			Username: "oauth-link",
			OAuthID:  "google-oauth-id-456",
		})
		if !errors.Is(err, ErrAccountExistsMustLink) {
			t.Fatalf("expected ErrAccountExistsMustLink, got %v", err)
		}
	})
}

func TestLoginWithExternalIdentity_ReturnsExistingProviderUser(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		existing, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "oauth-existing@example.com",
			Username: "oauth-existing",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}

		oauthID := "google-oauth-id-existing"
		_, err = tdb.Store.CreateAuthProvider(ctx, domain.AuthProvider{
			UserID:         existing.ID,
			Provider:       domain.ProviderGoogle,
			ProviderUserID: &oauthID,
		})
		if err != nil {
			t.Fatalf("CreateAuthProvider failed: %v", err)
		}

		svc := New(Config{
			Store:         tdb.Store,
			Tx:            tdb.Tx,
			AccessPolicy:  staticAccessPolicy{allowed: true},
			Organizations: testOrganizations(tdb),
			OAuthProviders: oauth.OAuthProviders{
				Providers: []oauth.OAuthProvider{
					oauth.NewOAuthProvider(domain.ProviderGoogle, "test-google-client-id", "http://localhost:3000"),
				},
			},
		})

		user, err := svc.Login(ctx, LoginInput{
			Provider: domain.ProviderGoogle,
			Email:    existing.Email,
			Username: existing.Username,
			OAuthID:  oauthID,
		})
		if err != nil {
			t.Fatalf("Login failed: %v", err)
		}
		if user.ID != existing.ID {
			t.Fatalf("expected user id %q, got %q", existing.ID, user.ID)
		}
	})
}

func TestChangeUsername_Succeeds(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "change-username@example.com",
			Username: "old-username",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}

		svc := New(Config{
			Store: tdb.Store,
			Tx:    tdb.Tx,
		})

		err = svc.ChangeUsername(ctx, user.ID, "new_username")
		if err != nil {
			t.Fatalf("ChangeUsername failed: %v", err)
		}

		updated, err := tdb.Store.GetUserByID(ctx, user.ID)
		if err != nil {
			t.Fatalf("GetUserByID failed: %v", err)
		}
		if updated.Username != "new_username" {
			t.Fatalf("expected username new_username, got %q", updated.Username)
		}
	})
}

func TestChangeUsername_InvalidUsername(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "invalid-username@example.com",
			Username: "valid-user",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}

		svc := New(Config{
			Store: tdb.Store,
			Tx:    tdb.Tx,
		})

		err = svc.ChangeUsername(ctx, user.ID, "x")
		if !errors.Is(err, ErrInvalidUsername) {
			t.Fatalf("expected ErrInvalidUsername, got %v", err)
		}
	})
}

func TestChangeUsername_UsernameTaken(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		_, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "taken-a@example.com",
			Username: "taken-name",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}

		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "taken-b@example.com",
			Username: "other-name",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}

		svc := New(Config{
			Store: tdb.Store,
			Tx:    tdb.Tx,
		})

		err = svc.ChangeUsername(ctx, user.ID, "taken-name")
		if !errors.Is(err, ErrUsernameTaken) {
			t.Fatalf("expected ErrUsernameTaken, got %v", err)
		}
	})
}

func TestDisableUser(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "disable@example.com",
			Username: "disable-user",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}

		svc := New(Config{
			Store: tdb.Store,
			Tx:    tdb.Tx,
		})

		err = svc.DisableUser(ctx, user.ID)
		if err != nil {
			t.Fatalf("DisableUser failed: %v", err)
		}

		updated, err := tdb.Store.GetUserByID(ctx, user.ID)
		if err != nil {
			t.Fatalf("GetUserByID failed: %v", err)
		}
		if updated.DisabledAt == nil {
			t.Fatal("expected DisabledAt to be set")
		}
	})
}

func TestDeleteUser_RemovesUser(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "delete-user@example.com",
			Username: "delete-user",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}

		svc := New(Config{
			Store: tdb.Store,
			Tx:    tdb.Tx,
		})

		err = svc.DeleteUser(ctx, user.ID)
		if err != nil {
			t.Fatalf("DeleteUser failed: %v", err)
		}

		_, err = tdb.Store.GetUserByID(ctx, user.ID)
		if err == nil {
			t.Fatal("expected deleted user lookup to fail")
		}
	})
}

func TestDeleteUserRejectsLastActiveAdmin(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "delete-last-admin@example.com",
			Username: "delete-last-admin",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}
		if err := tdb.Store.AddUserPlatformRoleByName(ctx, user.ID, roles.DBAdminRoleName); err != nil {
			t.Fatalf("AddUserPlatformRoleByName failed: %v", err)
		}

		svc := New(Config{
			Store: tdb.Store,
			Tx:    tdb.Tx,
		})

		err = svc.DeleteUser(ctx, user.ID)
		if !errors.Is(err, ErrCannotDeleteLastAdmin) {
			t.Fatalf("expected ErrCannotDeleteLastAdmin, got %v", err)
		}

		if _, err := tdb.Store.GetUserByID(ctx, user.ID); err != nil {
			t.Fatalf("expected last admin to remain, got %v", err)
		}
	})
}

func TestDeleteUserAllowsAdminWhenAnotherActiveAdminExists(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "delete-admin-target@example.com",
			Username: "delete-admin-target",
		})
		if err != nil {
			t.Fatalf("CreateUser target failed: %v", err)
		}
		otherAdmin, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "delete-admin-other@example.com",
			Username: "delete-admin-other",
		})
		if err != nil {
			t.Fatalf("CreateUser other admin failed: %v", err)
		}
		for _, id := range []uuid.UUID{user.ID, otherAdmin.ID} {
			if err := tdb.Store.AddUserPlatformRoleByName(ctx, id, roles.DBAdminRoleName); err != nil {
				t.Fatalf("AddUserPlatformRoleByName failed: %v", err)
			}
		}

		svc := New(Config{
			Store: tdb.Store,
			Tx:    tdb.Tx,
		})

		if err := svc.DeleteUser(ctx, user.ID); err != nil {
			t.Fatalf("DeleteUser failed: %v", err)
		}

		if _, err := tdb.Store.GetUserByID(ctx, user.ID); err == nil {
			t.Fatal("expected deleted admin lookup to fail")
		}
		if _, err := tdb.Store.GetUserByID(ctx, otherAdmin.ID); err != nil {
			t.Fatalf("expected other admin to remain, got %v", err)
		}
	})
}

func createPasswordUser(
	t *testing.T,
	ctx context.Context,
	tdb *testutil.TestDB,
	email string,
	username string,
	passwordHash string,
) domain.User {
	t.Helper()

	user, err := tdb.Store.CreateUser(ctx, domain.User{
		Email:    email,
		Username: username,
	})
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	_, err = tdb.Store.CreateAuthProvider(ctx, domain.AuthProvider{
		UserID:       user.ID,
		Provider:     domain.ProviderPassword,
		PasswordHash: &passwordHash,
	})
	if err != nil {
		t.Fatalf("CreateAuthProvider failed: %v", err)
	}

	return user
}

func TestUnlinkAuthProvider_BlockedWhenOnlyProviderAndNoPasskey(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user := createPasswordUser(t, ctx, tdb, "unlink-only-provider@example.com", "unlink-only-provider", "hashed-password")
		svc := New(Config{
			Store: tdb.Store,
			Tx:    tdb.Tx,
		})

		err := svc.UnlinkAuthProvider(ctx, user.ID, domain.ProviderPassword)
		if !errors.Is(err, ErrCannotRemoveLastAuthMethod) {
			t.Fatalf("expected ErrCannotRemoveLastAuthMethod, got %v", err)
		}
	})
}

func TestUnlinkAuthProvider_AllowedWhenPasskeyExists(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user := createPasswordUser(t, ctx, tdb, "unlink-with-passkey@example.com", "unlink-with-passkey", "hashed-password")
		_, err := tdb.Store.CreatePasskey(ctx, domain.Passkey{
			UserID:       user.ID,
			CredentialID: []byte("unlink-provider-passkey"),
			PublicKey:    []byte("public-key"),
			Name:         "Passkey",
		})
		if err != nil {
			t.Fatalf("CreatePasskey failed: %v", err)
		}

		svc := New(Config{
			Store: tdb.Store,
			Tx:    tdb.Tx,
		})

		if err := svc.UnlinkAuthProvider(ctx, user.ID, domain.ProviderPassword); err != nil {
			t.Fatalf("UnlinkAuthProvider failed: %v", err)
		}

		count, err := tdb.Store.CountAuthMethods(ctx, user.ID)
		if err != nil {
			t.Fatalf("CountAuthMethods failed: %v", err)
		}
		if count != 1 {
			t.Fatalf("expected only passkey to remain, got %d auth methods", count)
		}
	})
}

func TestGetUser_NotFound(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		svc := New(Config{
			Store: tdb.Store,
			Tx:    tdb.Tx,
		})

		_, err := svc.GetUser(ctx, uuid.New())
		if err == nil {
			t.Fatal("expected error for missing user")
		}
	})
}

func TestSignup_DuplicateEmailReturnsUserAlreadyExists(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		_, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "duplicate@example.com",
			Username: "existing-user",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}

		svc := New(Config{
			Store:         tdb.Store,
			Tx:            tdb.Tx,
			AccessPolicy:  staticAccessPolicy{allowed: true},
			Organizations: testOrganizations(tdb),
		})

		_, err = svc.Signup(ctx, SignupInput{
			Provider:     domain.ProviderPassword,
			Email:        "duplicate@example.com",
			Username:     "new-user",
			PasswordHash: "hashed-password",
		})
		if !errors.Is(err, ErrUserAlreadyExists) {
			t.Fatalf("expected ErrUserAlreadyExists, got %v", err)
		}
	})
}

func TestSignup_AccessPolicyError(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	policyErr := errors.New("policy failure")

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		svc := New(Config{
			Store:        tdb.Store,
			Tx:           tdb.Tx,
			AccessPolicy: staticAccessPolicy{err: policyErr},
		})

		_, err := svc.Signup(ctx, SignupInput{
			Provider:     domain.ProviderPassword,
			Email:        "signup-policy-error@example.com",
			Username:     "signup-policy-error",
			PasswordHash: "hashed-password",
		})
		if !errors.Is(err, policyErr) {
			t.Fatalf("expected policy error %v, got %v", policyErr, err)
		}
	})
}

func TestLogin_AccessPolicyError(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	policyErr := errors.New("policy failure")

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		svc := New(Config{
			Store:        tdb.Store,
			Tx:           tdb.Tx,
			AccessPolicy: staticAccessPolicy{err: policyErr},
		})

		_, err := svc.Login(ctx, LoginInput{
			Provider: domain.ProviderPassword,
			Email:    "login-policy-error@example.com",
			Password: "irrelevant",
		})
		if !errors.Is(err, policyErr) {
			t.Fatalf("expected policy error %v, got %v", policyErr, err)
		}
	})
}

func TestLogin_UserNotFound(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		svc := New(Config{
			Store:        tdb.Store,
			Tx:           tdb.Tx,
			AccessPolicy: staticAccessPolicy{allowed: true},
		})

		_, err := svc.Login(ctx, LoginInput{
			Provider: domain.ProviderPassword,
			Email:    "missing-login@example.com",
			Password: "irrelevant",
		})
		if err == nil {
			t.Fatal("expected error for missing user")
		}
	})
}

func TestLogin_PasswordProviderMissing(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		_, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "oauth-only@example.com",
			Username: "oauth-only-user",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}

		svc := New(Config{
			Store:        tdb.Store,
			Tx:           tdb.Tx,
			AccessPolicy: staticAccessPolicy{allowed: true},
		})

		_, err = svc.Login(ctx, LoginInput{
			Provider: domain.ProviderPassword,
			Email:    "oauth-only@example.com",
			Password: "irrelevant",
		})
		if err == nil {
			t.Fatal("expected error when password auth provider is missing")
		}
	})
}

func TestStartAccountRecoveryProviderLink_CreatesPendingLinkForExistingEmail(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		passwordHash, err := Hash("correct-password")
		if err != nil {
			t.Fatalf("Hash failed: %v", err)
		}
		user := createPasswordUser(t, ctx, tdb, "collision@example.com", "collision-user", passwordHash)

		svc := New(Config{
			Store: tdb.Store,
			Tx:    tdb.Tx,
			OAuthProviders: oauth.OAuthProviders{
				Providers: []oauth.OAuthProvider{
					oauth.NewOAuthProvider(domain.ProviderGoogle, "test-google-client-id", "http://localhost:3000"),
				},
			},
		})

		link, err := svc.StartAccountRecoveryProviderLink(ctx, OAuthIdentityInput{
			Provider:              domain.ProviderGoogle,
			Email:                 "collision@example.com",
			ProviderUserID:        "google-collision-sub",
			ProviderEmailVerified: true,
		}, time.Date(2026, 5, 7, 8, 0, 0, 0, time.UTC))
		if err != nil {
			t.Fatalf("StartAccountRecoveryProviderLink failed: %v", err)
		}

		if link.UserID != user.ID {
			t.Fatalf("expected link user %q, got %q", user.ID, link.UserID)
		}
		if link.SessionID != nil {
			t.Fatal("expected unauthenticated collision link to have nil session")
		}
		if link.Purpose != domain.PendingProviderLinkPurposeAccountRecovery {
			t.Fatalf("expected account recovery purpose, got %q", link.Purpose)
		}
		if link.ProviderUserID == nil || *link.ProviderUserID != "google-collision-sub" {
			t.Fatalf("expected pending provider user id to be stored")
		}
	})
}

func TestCompleteAccountRecoveryProviderLinkWithPassword_LinksProvider(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		passwordHash, err := Hash("correct-password")
		if err != nil {
			t.Fatalf("Hash failed: %v", err)
		}
		user := createPasswordUser(t, ctx, tdb, "complete-collision@example.com", "complete-collision-user", passwordHash)

		svc := New(Config{
			Store: tdb.Store,
			Tx:    tdb.Tx,
			OAuthProviders: oauth.OAuthProviders{
				Providers: []oauth.OAuthProvider{
					oauth.NewOAuthProvider(domain.ProviderGoogle, "test-google-client-id", "http://localhost:3000"),
				},
			},
		})

		now := time.Date(2026, 5, 7, 8, 0, 0, 0, time.UTC)
		link, err := svc.StartAccountRecoveryProviderLink(ctx, OAuthIdentityInput{
			Provider:              domain.ProviderGoogle,
			Email:                 "complete-collision@example.com",
			ProviderUserID:        "google-complete-collision-sub",
			ProviderEmailVerified: true,
		}, now)
		if err != nil {
			t.Fatalf("StartAccountRecoveryProviderLink failed: %v", err)
		}

		got, err := svc.CompleteAccountRecoveryProviderLinkWithPassword(ctx, link.ID, "correct-password", now)
		if err != nil {
			t.Fatalf("CompleteAccountRecoveryProviderLinkWithPassword failed: %v", err)
		}
		if got.ID != user.ID {
			t.Fatalf("expected signed-in user %q, got %q", user.ID, got.ID)
		}

		provider, err := tdb.Store.GetAuthProviderByProviderAndProviderUserID(ctx, domain.ProviderGoogle, "google-complete-collision-sub")
		if err != nil {
			t.Fatalf("GetAuthProviderByProviderAndProviderUserID failed: %v", err)
		}
		if provider.UserID != user.ID {
			t.Fatalf("expected provider linked to %q, got %q", user.ID, provider.UserID)
		}

		_, err = svc.CompleteAccountRecoveryProviderLinkWithPassword(ctx, link.ID, "correct-password", now)
		if !errors.Is(err, ErrPendingProviderLinkExpired) && !errors.Is(err, ErrPendingProviderLinkInvalid) {
			t.Fatalf("expected consumed pending link to be rejected, got %v", err)
		}
	})
}

func TestLoginWithExternalIdentity_CreatesUserWhenMissing_PersistsUserAndProvider(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		svc := New(Config{
			Store:         tdb.Store,
			Tx:            tdb.Tx,
			AccessPolicy:  staticAccessPolicy{allowed: true},
			Organizations: testOrganizations(tdb),
			OAuthProviders: oauth.OAuthProviders{
				Providers: []oauth.OAuthProvider{
					oauth.NewOAuthProvider(domain.ProviderGoogle, "test-google-client-id", "http://localhost:3000"),
				},
			},
		})

		user, err := svc.Login(ctx, LoginInput{
			Provider: domain.ProviderGoogle,
			Email:    "oauth-new@example.com",
			Username: "oauth-new",
			OAuthID:  "google-oauth-id-123",
		})
		if err != nil {
			t.Fatalf("Login failed: %v", err)
		}

		dbUser, err := tdb.Store.GetUserByEmail(ctx, "oauth-new@example.com")
		if err != nil {
			t.Fatalf("expected user to be created in DB: %v", err)
		}
		if dbUser.ID != user.ID {
			t.Fatalf("expected DB user id %q, got %q", user.ID, dbUser.ID)
		}

		provider, err := tdb.Store.GetAuthProviderByProviderAndProviderUserID(
			ctx,
			domain.ProviderGoogle,
			"google-oauth-id-123",
		)
		if err != nil {
			t.Fatalf("expected auth provider to be created: %v", err)
		}
		if provider.UserID != user.ID {
			t.Fatalf("expected provider user_id %q, got %q", user.ID, provider.UserID)
		}
	})
}

func TestLoginWithExternalIdentity_GeneratesUsernameWhenEmpty(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		svc := New(Config{
			Store:         tdb.Store,
			Tx:            tdb.Tx,
			AccessPolicy:  staticAccessPolicy{allowed: true},
			Organizations: testOrganizations(tdb),
			OAuthProviders: oauth.OAuthProviders{
				Providers: []oauth.OAuthProvider{
					oauth.NewOAuthProvider(domain.ProviderGoogle, "test-google-client-id", "http://localhost:3000"),
				},
			},
		})

		user, err := svc.Login(ctx, LoginInput{
			Provider: domain.ProviderGoogle,
			Email:    "generated.username@example.com",
			Username: "",
			OAuthID:  "google-generated-username-id",
		})
		if err != nil {
			t.Fatalf("Login failed: %v", err)
		}
		if user.Username == "" {
			t.Fatal("expected generated username, got empty string")
		}

		dbUser, err := tdb.Store.GetUserByEmail(ctx, "generated.username@example.com")
		if err != nil {
			t.Fatalf("expected user to be created in DB: %v", err)
		}
		if dbUser.Username == "" {
			t.Fatal("expected persisted generated username, got empty string")
		}
	})
}

func TestLoginWithExternalIdentity_BlockedByAccessPolicy(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		svc := New(Config{
			Store:        tdb.Store,
			Tx:           tdb.Tx,
			AccessPolicy: staticAccessPolicy{allowed: false},
			OAuthProviders: oauth.OAuthProviders{
				Providers: []oauth.OAuthProvider{
					oauth.NewOAuthProvider(domain.ProviderGoogle, "test-google-client-id", "http://localhost:3000"),
				},
			},
		})

		_, err := svc.Login(ctx, LoginInput{
			Provider: domain.ProviderGoogle,
			Email:    "oauth-blocked@example.com",
			Username: "oauth-blocked",
			OAuthID:  "google-oauth-blocked-id",
		})
		if !errors.Is(err, ErrEmailNotAllowed) {
			t.Fatalf("expected ErrEmailNotAllowed, got %v", err)
		}
	})
}
