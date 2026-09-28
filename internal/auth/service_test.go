package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/accesspolicy"
	"github.com/authara-org/authara/internal/cache"
	"github.com/authara-org/authara/internal/challenge"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/email"
	"github.com/authara-org/authara/internal/oauth"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/securityevent"
	"github.com/authara-org/authara/internal/session/roles"
	"github.com/authara-org/authara/internal/session/token"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/authara-org/authara/internal/webhook"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"
)

type staticAccessPolicy struct {
	allowed bool
	err     error
}

type recordingAccessPolicy struct {
	allowed   bool
	lastEmail string
}

type publisherFunc func(context.Context, webhook.Envelope) error

type failingPasswordChangedRecorder struct {
	securityevent.NoopRecorder
	err error
}

func (r failingPasswordChangedRecorder) CredentialPasswordChanged(context.Context, securityevent.Credential) error {
	return r.err
}

type authServiceTestCache struct {
	values    map[string][]byte
	setErr    error
	setCalls  int
	failAfter int
}

func (c *authServiceTestCache) Get(_ context.Context, key string) ([]byte, error) {
	value, ok := c.values[key]
	if !ok {
		return nil, cache.ErrMiss
	}
	return value, nil
}

func (c *authServiceTestCache) GetMany(_ context.Context, keys ...string) ([][]byte, error) {
	values := make([][]byte, len(keys))
	for i, key := range keys {
		values[i] = c.values[key]
	}
	return values, nil
}

func (c *authServiceTestCache) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	c.setCalls++
	if c.setErr != nil && (c.failAfter == 0 || c.setCalls > c.failAfter) {
		return c.setErr
	}
	if c.values == nil {
		c.values = make(map[string][]byte)
	}
	c.values[key] = append([]byte(nil), value...)
	return nil
}

func (c *authServiceTestCache) SetMaxInt64(ctx context.Context, key string, value int64, ttl time.Duration) error {
	return c.Set(ctx, key, []byte(strconv.FormatInt(value, 10)), ttl)
}

func (c *authServiceTestCache) Delete(_ context.Context, key string) error {
	delete(c.values, key)
	return nil
}

func (c *authServiceTestCache) Close() error { return nil }

func (f publisherFunc) Publish(ctx context.Context, evt webhook.Envelope) error {
	return f(ctx, evt)
}

func (p staticAccessPolicy) IsEmailAllowed(ctx context.Context, email string) (bool, error) {
	if p.err != nil {
		return false, p.err
	}
	return p.allowed, nil
}

func (p *recordingAccessPolicy) IsEmailAllowed(_ context.Context, email string) (bool, error) {
	p.lastEmail = email
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
		if got := testutil.CountEmailJobs(t, ctx, user.Email, domain.EmailTemplateAccountCreated); got != 1 {
			t.Fatalf("account-created email jobs = %d, want 1", got)
		}
		data := testutil.LatestEmailTemplateData(t, ctx, user.Email, domain.EmailTemplateAccountCreated)
		if data[email.TemplateVariableUsername] != user.Username || data[email.TemplateVariableAuthMethod] != string(domain.ProviderPassword) {
			t.Fatalf("unexpected account-created template data: %#v", data)
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

func TestLogin_WithPassword_UpgradesOutdatedHash(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		password := "an old but valid password"
		salt := []byte("1234567890abcdef")
		outdatedHash := encodeHash(2, 32*1024, 2, salt, argon2.IDKey([]byte(password), salt, 2, 32*1024, 2, 24))
		user := createPasswordUser(t, ctx, tdb, "rehash-login@example.com", "rehash-login", outdatedHash)

		svc := New(Config{
			Store:        tdb.Store,
			Tx:           tdb.Tx,
			AccessPolicy: staticAccessPolicy{allowed: true},
		})
		if _, err := svc.Login(ctx, LoginInput{
			Provider: domain.ProviderPassword,
			Email:    user.Email,
			Password: password,
		}); err != nil {
			t.Fatalf("Login failed: %v", err)
		}

		provider, err := tdb.Store.GetAuthProviderByMethodAndUserID(ctx, domain.ProviderPassword, user.ID)
		if err != nil {
			t.Fatal(err)
		}
		if provider.PasswordHash == nil || *provider.PasswordHash == outdatedHash {
			t.Fatal("successful login did not replace the outdated hash")
		}
		verification, err := VerifyPasswordHash(password, *provider.PasswordHash)
		if err != nil {
			t.Fatal(err)
		}
		if !verification.Valid || verification.NeedsRehash {
			t.Fatalf("upgraded hash verification = %+v", verification)
		}
	})
}

func TestLogin_WithPassword_DoesNotUpgradeBeforeSuccessfulVerification(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		password := "an old but valid password"
		salt := []byte("1234567890abcdef")
		outdatedHash := encodeHash(2, 32*1024, 2, salt, argon2.IDKey([]byte(password), salt, 2, 32*1024, 2, 24))
		user := createPasswordUser(t, ctx, tdb, "no-rehash@example.com", "no-rehash", outdatedHash)

		svc := New(Config{
			Store: tdb.Store, Tx: tdb.Tx, AccessPolicy: staticAccessPolicy{allowed: true},
			SecurityEvents: securityevent.NewStandard(tdb.Store, 180*24*time.Hour),
		})
		before, err := tdb.Store.QuerySecurityEvents(ctx, store.SecurityEventFilter{
			Type:    domain.SecurityEventAuthenticationLogin,
			Outcome: domain.SecurityEventOutcomeDenied,
		})
		if err != nil {
			t.Fatal(err)
		}
		_, err = svc.Login(ctx, LoginInput{Provider: domain.ProviderPassword, Email: user.Email, Password: "wrong password"})
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("Login error = %v, want ErrInvalidCredentials", err)
		}
		after, err := tdb.Store.QuerySecurityEvents(ctx, store.SecurityEventFilter{
			Type:    domain.SecurityEventAuthenticationLogin,
			Outcome: domain.SecurityEventOutcomeDenied,
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(after) != len(before)+1 {
			t.Fatalf("denied login events = %d, want %d", len(after), len(before)+1)
		}
		event := after[0]
		if event.ReasonCode != domain.SecurityEventReasonInvalidCredentials || event.AuthenticationMethod != domain.AuthenticationMethodPassword || event.UserID != nil {
			t.Fatalf("unexpected denied login event: %+v", event)
		}

		provider, err := tdb.Store.GetAuthProviderByMethodAndUserID(ctx, domain.ProviderPassword, user.ID)
		if err != nil {
			t.Fatal(err)
		}
		if provider.PasswordHash == nil || *provider.PasswordHash != outdatedHash {
			t.Fatal("failed login changed the stored password hash")
		}
	})
}

func TestPasswordHashCompareAndSwapDoesNotOverwriteConcurrentChange(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user := createPasswordUser(t, ctx, tdb, "cas-password@example.com", "cas-password", "newer-hash")
		updated, err := tdb.Store.CompareAndSwapPasswordHash(ctx, user.ID, "stale-hash", "replacement-hash")
		if err != nil {
			t.Fatal(err)
		}
		if updated {
			t.Fatal("stale expected hash unexpectedly replaced a newer credential")
		}
		provider, err := tdb.Store.GetAuthProviderByMethodAndUserID(ctx, domain.ProviderPassword, user.ID)
		if err != nil {
			t.Fatal(err)
		}
		if provider.PasswordHash == nil || *provider.PasswordHash != "newer-hash" {
			t.Fatalf("stored password hash = %v, want newer-hash", provider.PasswordHash)
		}
	})
}

func TestLogin_WithUsername_Succeeds(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		passwordHash, err := Hash("super-secret")
		if err != nil {
			t.Fatalf("Hash failed: %v", err)
		}

		user := createPasswordUser(t, ctx, tdb, "username-login@example.com", "UsernameLogin", passwordHash)
		policy := &recordingAccessPolicy{allowed: true}
		svc := New(Config{
			Store:        tdb.Store,
			Tx:           tdb.Tx,
			AccessPolicy: policy,
		})

		got, err := svc.Login(ctx, LoginInput{
			Provider:   domain.ProviderPassword,
			Identifier: "usernamelogin",
			Password:   "super-secret",
		})
		if err != nil {
			t.Fatalf("Login failed: %v", err)
		}
		if got.ID != user.ID {
			t.Fatalf("expected user id %q, got %q", user.ID, got.ID)
		}
		if policy.lastEmail != user.Email {
			t.Fatalf("expected access policy email %q, got %q", user.Email, policy.lastEmail)
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
		passwordHash, err := Hash("correct-password")
		if err != nil {
			t.Fatalf("Hash failed: %v", err)
		}
		createPasswordUser(t, ctx, tdb, "blocked@example.com", "blocked-user", passwordHash)

		svc := New(Config{
			Store:        tdb.Store,
			Tx:           tdb.Tx,
			AccessPolicy: staticAccessPolicy{allowed: false},
		})

		_, err = svc.Login(ctx, LoginInput{
			Provider: domain.ProviderPassword,
			Email:    "blocked@example.com",
			Password: "correct-password",
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
		if got := testutil.CountEmailJobs(t, ctx, user.Email, domain.EmailTemplateAccountCreated); got != 1 {
			t.Fatalf("OAuth account-created email jobs = %d, want 1", got)
		}
		data := testutil.LatestEmailTemplateData(t, ctx, user.Email, domain.EmailTemplateAccountCreated)
		if data[email.TemplateVariableAuthMethod] != string(domain.ProviderGoogle) {
			t.Fatalf("unexpected OAuth account-created template data: %#v", data)
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
		if got := testutil.CountEmailJobs(t, ctx, existing.Email, domain.EmailTemplateAccountCreated); got != 0 {
			t.Fatalf("account-created email jobs for existing login = %d, want 0", got)
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
		if got := testutil.CountEmailJobs(t, ctx, user.Email, domain.EmailTemplateAccountDisabled); got != 1 {
			t.Fatalf("account-disabled email jobs = %d, want 1", got)
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

func TestDeleteUserRemovesOwnedPersonalOrganization(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user, err := tdb.Store.CreateUser(ctx, domain.User{Email: "delete-personal@example.com", Username: "delete-personal"})
		if err != nil {
			t.Fatal(err)
		}
		orgs := organization.New(organization.Config{Store: tdb.Store, Tx: tdb.Tx, Mode: organization.OrgModeMulti})
		org, _, _, err := orgs.EnsureInitialOrganization(ctx, user, organization.SignupSourceDirect)
		if err != nil {
			t.Fatal(err)
		}

		// The invariant belongs to DeleteUser itself, even when a caller does not
		// inject the organization service explicitly.
		svc := New(Config{Store: tdb.Store, Tx: tdb.Tx})
		if err := svc.DeleteUser(ctx, user.ID); err != nil {
			t.Fatalf("DeleteUser failed: %v", err)
		}
		if _, err := tdb.Store.GetOrganizationByID(ctx, org.ID); !errors.Is(err, store.ErrOrganizationNotFound) {
			t.Fatalf("expected personal organization deletion, got %v", err)
		}
	})
}

func TestDeleteUserCannotBypassOrganizationLifecycle(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		owner, err := tdb.Store.CreateUser(ctx, domain.User{Email: "delete-team-owner@example.com", Username: "delete-team-owner"})
		if err != nil {
			t.Fatal(err)
		}
		orgs := organization.New(organization.Config{Store: tdb.Store, Tx: tdb.Tx, Mode: organization.OrgModeSingle})
		org, _, _, err := orgs.EnsureInitialOrganization(ctx, owner, organization.SignupSourceDirect)
		if err != nil {
			t.Fatal(err)
		}
		svc := New(Config{Store: tdb.Store, Tx: tdb.Tx})

		err = svc.DeleteUser(ctx, owner.ID)
		if !errors.Is(err, organization.ErrLastOrganizationMember) {
			t.Fatalf("expected last-member error, got %v", err)
		}
		if _, err := tdb.Store.GetUserByID(ctx, owner.ID); err != nil {
			t.Fatalf("expected user to remain: %v", err)
		}
		if _, err := tdb.Store.GetOrganizationByID(ctx, org.ID); err != nil {
			t.Fatalf("expected organization to remain: %v", err)
		}
	})
}

func TestDeleteUserRemovesNonCreatorPersonalMembershipInMultiMode(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		creator, err := tdb.Store.CreateUser(ctx, domain.User{Email: "delete-personal-creator@example.com", Username: "delete-personal-creator"})
		if err != nil {
			t.Fatal(err)
		}
		user, err := tdb.Store.CreateUser(ctx, domain.User{Email: "delete-personal-member@example.com", Username: "delete-personal-member"})
		if err != nil {
			t.Fatal(err)
		}
		org, _, err := tdb.Store.EnsureOrganizationForUser(ctx, creator.ID, "Creator Personal", domain.OrganizationKindPersonal)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tdb.Store.CreateOrganizationMembership(ctx, domain.OrganizationMembership{
			OrganizationID: org.ID,
			UserID:         user.ID,
			Role:           domain.OrganizationRoleMember,
		}); err != nil {
			t.Fatal(err)
		}

		orgs := organization.New(organization.Config{Store: tdb.Store, Tx: tdb.Tx, Mode: organization.OrgModeMulti})
		svc := New(Config{Store: tdb.Store, Tx: tdb.Tx, Organizations: orgs})
		if err := svc.DeleteUser(ctx, user.ID); err != nil {
			t.Fatalf("DeleteUser failed: %v", err)
		}
		if _, err := tdb.Store.GetOrganizationByID(ctx, org.ID); err != nil {
			t.Fatalf("expected creator personal organization to remain: %v", err)
		}
		if _, err := tdb.Store.GetUserByID(ctx, user.ID); !errors.Is(err, store.ErrUserNotFound) {
			t.Fatalf("expected non-creator user deletion, got %v", err)
		}
	})
}

func TestDeleteUserLocksMembershipSnapshot(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	ctx := context.Background()
	suffix := uuid.NewString()
	user, err := tdb.Store.CreateUser(ctx, domain.User{
		Email:    "delete-lock-" + suffix + "@example.com",
		Username: "delete-lock-" + suffix,
	})
	if err != nil {
		t.Fatal(err)
	}
	setupOrganizations := organization.New(organization.Config{Store: tdb.Store, Tx: tdb.Tx, Mode: organization.OrgModeMulti})
	if _, _, _, err := setupOrganizations.EnsureInitialOrganization(ctx, user, organization.SignupSourceDirect); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = tdb.Store.DeleteUser(context.Background(), user.ID)
	})

	createOrganizations := organization.New(organization.Config{Store: tdb.Store, Tx: tdb.Tx, Mode: organization.OrgModeMulti})
	createDone := make(chan error, 1)
	pub := publisherFunc(func(_ context.Context, evt webhook.Envelope) error {
		if evt.Type != webhook.EventOrganizationDeleted {
			return nil
		}
		go func() {
			_, _, err := createOrganizations.CreateOrganization(context.Background(), organization.CreateOrganizationInput{
				Name:            "Concurrent Organization",
				CreatedByUserID: user.ID,
			})
			createDone <- err
		}()
		select {
		case <-createDone:
			return errors.New("organization creation completed before user deletion released its lock")
		case <-time.After(100 * time.Millisecond):
			return nil
		}
	})
	deleteOrganizations := organization.New(organization.Config{
		Store:            tdb.Store,
		Tx:               tdb.Tx,
		Mode:             organization.OrgModeMulti,
		WebhookPublisher: pub,
	})
	svc := New(Config{Store: tdb.Store, Tx: tdb.Tx, Organizations: deleteOrganizations, WebhookPublisher: pub})
	if err := svc.DeleteUser(ctx, user.ID); err != nil {
		t.Fatalf("DeleteUser failed: %v", err)
	}
	select {
	case err := <-createDone:
		if err == nil {
			t.Fatal("expected concurrent organization creation to fail after user deletion")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("concurrent organization creation did not finish after user deletion")
	}
}

func TestDeleteUserUsesMembershipRoleAfterOrganizationLock(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	ctx := context.Background()
	suffix := uuid.NewString()
	owner, err := tdb.Store.CreateUser(ctx, domain.User{
		Email:    "fresh-role-owner-" + suffix + "@example.com",
		Username: "fresh-role-owner-" + suffix,
	})
	if err != nil {
		t.Fatal(err)
	}
	user, err := tdb.Store.CreateUser(ctx, domain.User{
		Email:    "fresh-role-user-" + suffix + "@example.com",
		Username: "fresh-role-user-" + suffix,
	})
	if err != nil {
		t.Fatal(err)
	}
	org, _, err := tdb.Store.EnsureOrganizationForUser(ctx, owner.ID, "Fresh Role", domain.OrganizationKindTeam)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tdb.Store.CreateOrganizationMembership(ctx, domain.OrganizationMembership{
		OrganizationID: org.ID,
		UserID:         user.ID,
		Role:           domain.OrganizationRoleMember,
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = tdb.Store.DeleteOrganization(context.Background(), org.ID)
		_ = tdb.Store.DeleteUser(context.Background(), user.ID)
		_ = tdb.Store.DeleteUser(context.Background(), owner.ID)
	})

	lockCtx, cancel, err := tdb.Tx.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	if _, err := tdb.Store.GetOrganizationByIDForUpdate(lockCtx, org.ID); err != nil {
		t.Fatal(err)
	}

	organizations := organization.New(organization.Config{Store: tdb.Store, Tx: tdb.Tx, Mode: organization.OrgModeMulti})
	svc := New(Config{Store: tdb.Store, Tx: tdb.Tx, Organizations: organizations})
	done := make(chan error, 1)
	go func() { done <- svc.DeleteUser(ctx, user.ID) }()
	select {
	case err := <-done:
		t.Fatalf("user deletion completed before organization lock was released: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	if _, err := tdb.Store.UpdateOrganizationMembershipRole(lockCtx, org.ID, owner.ID, domain.OrganizationRoleAdmin); err != nil {
		t.Fatal(err)
	}
	if _, err := tdb.Store.UpdateOrganizationMembershipRole(lockCtx, org.ID, user.ID, domain.OrganizationRoleOwner); err != nil {
		t.Fatal(err)
	}
	if err := tdb.Tx.Commit(lockCtx); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-done:
		if !errors.Is(err, organization.ErrLastOrganizationOwner) {
			t.Fatalf("expected current owner role to block deletion, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("user deletion did not finish after organization lock was released")
	}
	if _, err := tdb.Store.GetUserByID(ctx, user.ID); err != nil {
		t.Fatalf("expected user to remain: %v", err)
	}
}

func TestOwnershipTransferPreventsConcurrentNewOwnerDeletion(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	ctx := context.Background()
	suffix := uuid.NewString()
	owner, err := tdb.Store.CreateUser(ctx, domain.User{
		Email:    "locked-transfer-owner-" + suffix + "@example.com",
		Username: "locked-transfer-owner-" + suffix,
	})
	if err != nil {
		t.Fatal(err)
	}
	newOwner, err := tdb.Store.CreateUser(ctx, domain.User{
		Email:    "locked-transfer-target-" + suffix + "@example.com",
		Username: "locked-transfer-target-" + suffix,
	})
	if err != nil {
		t.Fatal(err)
	}
	org, _, err := tdb.Store.EnsureOrganizationForUser(ctx, owner.ID, "Locked Transfer", domain.OrganizationKindTeam)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tdb.Store.CreateOrganizationMembership(ctx, domain.OrganizationMembership{
		OrganizationID: org.ID,
		UserID:         newOwner.ID,
		Role:           domain.OrganizationRoleMember,
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = tdb.Store.DeleteOrganization(context.Background(), org.ID)
		_ = tdb.Store.DeleteUser(context.Background(), newOwner.ID)
		_ = tdb.Store.DeleteUser(context.Background(), owner.ID)
	})

	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	pub := publisherFunc(func(_ context.Context, evt webhook.Envelope) error {
		if evt.Type == webhook.EventOrganizationMembershipUpdated {
			once.Do(func() { close(entered) })
			<-release
		}
		return nil
	})
	organizations := organization.New(organization.Config{
		Store:            tdb.Store,
		Tx:               tdb.Tx,
		Mode:             organization.OrgModeMulti,
		WebhookPublisher: pub,
	})
	transferDone := make(chan error, 1)
	go func() {
		transferDone <- organizations.TransferOrganizationOwnership(ctx, organization.TransferOrganizationOwnershipInput{
			OrganizationID: org.ID,
			ActorUserID:    owner.ID,
			NewOwnerUserID: newOwner.ID,
		})
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("ownership transfer did not reach its commit boundary")
	}

	deleteDone := make(chan error, 1)
	deleteService := New(Config{Store: tdb.Store, Tx: tdb.Tx, Organizations: organizations})
	go func() { deleteDone <- deleteService.DeleteUser(ctx, newOwner.ID) }()
	select {
	case err := <-deleteDone:
		t.Fatalf("new owner deletion completed before transfer released its user lock: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	if err := <-transferDone; err != nil {
		t.Fatalf("ownership transfer failed: %v", err)
	}
	select {
	case err := <-deleteDone:
		if !errors.Is(err, organization.ErrLastOrganizationOwner) {
			t.Fatalf("expected new owner deletion to be rejected, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("new owner deletion did not finish after transfer committed")
	}
}

func TestDeleteUser_RemovesEmailReferences(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		owner, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "delete-invite-owner@example.com",
			Username: "delete-invite-owner",
		})
		if err != nil {
			t.Fatalf("CreateUser owner failed: %v", err)
		}
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "delete-invite-user@example.com",
			Username: "delete-invite-user",
		})
		if err != nil {
			t.Fatalf("CreateUser user failed: %v", err)
		}
		org, _, err := tdb.Store.EnsureOrganizationForUser(ctx, owner.ID, owner.Username, domain.OrganizationKindTeam)
		if err != nil {
			t.Fatalf("EnsureOrganizationForUser failed: %v", err)
		}
		invitation, err := tdb.Store.CreateOrganizationInvitation(ctx, domain.OrganizationInvitation{
			OrganizationID:  org.ID,
			Email:           user.Email,
			Role:            domain.OrganizationRoleMember,
			TokenHash:       "delete-invite-user-token",
			InvitedByUserID: &owner.ID,
			ExpiresAt:       time.Now().Add(time.Hour),
		})
		if err != nil {
			t.Fatalf("CreateOrganizationInvitation failed: %v", err)
		}
		if err := tdb.Store.MarkOrganizationInvitationAccepted(ctx, invitation.ID, user.ID, time.Now()); err != nil {
			t.Fatalf("MarkOrganizationInvitationAccepted failed: %v", err)
		}
		if _, err := tdb.Store.CreateOrganizationMembership(ctx, domain.OrganizationMembership{
			OrganizationID: org.ID,
			UserID:         user.ID,
			Role:           domain.OrganizationRoleMember,
		}); err != nil {
			t.Fatalf("CreateOrganizationMembership failed: %v", err)
		}
		if err := tdb.Store.CreateAllowedEmail(ctx, domain.AllowedEmail{Email: user.Email}); err != nil {
			t.Fatalf("CreateAllowedEmail failed: %v", err)
		}
		if _, err := tdb.Store.CreateEmailJob(ctx, domain.EmailJob{
			ToEmail:       user.Email,
			Template:      domain.EmailTemplateOrganizationInvite,
			Status:        domain.EmailJobStatusPending,
			NextAttemptAt: time.Now(),
		}); err != nil {
			t.Fatalf("CreateEmailJob failed: %v", err)
		}
		targetEmail := user.Email
		if _, err := tdb.Store.CreateAdminAuditEvent(ctx, domain.AdminAuditEvent{
			Action:       "test",
			TargetUserID: &user.ID,
			TargetEmail:  &targetEmail,
			Metadata:     json.RawMessage(`{}`),
		}); err != nil {
			t.Fatalf("CreateAdminAuditEvent failed: %v", err)
		}
		createPendingEmailReferences(t, ctx, user.ID, org.ID, user.Email)

		svc := New(Config{
			Store: tdb.Store,
			Tx:    tdb.Tx,
		})

		if err := svc.DeleteUser(ctx, user.ID); err != nil {
			t.Fatalf("DeleteUser failed: %v", err)
		}

		if _, err := tdb.Store.GetOrganizationInvitationByID(ctx, invitation.ID); !errors.Is(err, store.ErrOrganizationInvitationNotFound) {
			t.Fatalf("expected invitation email history to be removed, got %v", err)
		}
		allowed, err := tdb.Store.IsEmailAllowed(ctx, user.Email)
		if err != nil {
			t.Fatalf("IsEmailAllowed failed: %v", err)
		}
		if allowed {
			t.Fatal("expected deleted user email to be removed from allowlist")
		}
		if refs := countEmailReferences(t, ctx, user.Email); refs != 0 {
			t.Fatalf("expected deleted user email to be removed from direct references, got %d", refs)
		}
	})
}

type txQueryer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func queryerFromTxContext(t *testing.T, ctx context.Context) txQueryer {
	t.Helper()

	q, ok := ctx.Value(store.DbKey).(txQueryer)
	if !ok {
		t.Fatal("expected transaction queryer in context")
	}
	return q
}

func createPendingEmailReferences(t *testing.T, ctx context.Context, userID, organizationID uuid.UUID, email string) {
	t.Helper()

	q := queryerFromTxContext(t, ctx)
	var challengeID uuid.UUID
	if err := q.QueryRowContext(ctx, `
		INSERT INTO challenges (purpose, email, expires_at)
		VALUES ('signup', $1, now() + interval '1 hour')
		RETURNING id
	`, email).Scan(&challengeID); err != nil {
		t.Fatalf("insert challenge failed: %v", err)
	}
	if _, err := q.ExecContext(ctx, `
		INSERT INTO pending_signup_actions (challenge_id, email, username, password_hash)
		VALUES ($1, $2, 'deleted-email-pending', 'hash')
	`, challengeID, email); err != nil {
		t.Fatalf("insert pending signup failed: %v", err)
	}
	var sessionID uuid.UUID
	if err := q.QueryRowContext(ctx, `
		INSERT INTO sessions (user_id, active_organization_id, expires_at, user_agent)
		VALUES ($1, $2, now() + interval '1 hour', 'delete-email-reference')
		RETURNING id
	`, userID, organizationID).Scan(&sessionID); err != nil {
		t.Fatalf("insert session failed: %v", err)
	}
	if _, err := q.ExecContext(ctx, `
		INSERT INTO pending_email_changes (challenge_id, user_id, initiating_session_id, old_email, new_email)
		VALUES ($1, $2, $3, $4, $4)
	`, challengeID, userID, sessionID, email); err != nil {
		t.Fatalf("insert pending email change failed: %v", err)
	}
	if _, err := q.ExecContext(ctx, `
		INSERT INTO pending_provider_links (user_id, provider, expires_at, provider_email)
		VALUES ($1, 'google', now() + interval '1 hour', $2)
	`, userID, email); err != nil {
		t.Fatalf("insert pending provider link failed: %v", err)
	}
}

func countEmailReferences(t *testing.T, ctx context.Context, email string) int {
	t.Helper()

	var count int
	err := queryerFromTxContext(t, ctx).QueryRowContext(ctx, `
		SELECT
			(SELECT count(*) FROM organization_invitations WHERE lower(email) = lower($1)) +
			(SELECT count(*) FROM allowed_emails WHERE lower(email) = lower($1)) +
			(SELECT count(*) FROM email_jobs WHERE lower(to_email) = lower($1)) +
			(SELECT count(*) FROM challenges WHERE lower(email) = lower($1)) +
			(SELECT count(*) FROM pending_signup_actions WHERE lower(email) = lower($1)) +
			(SELECT count(*) FROM pending_email_changes WHERE lower(old_email) = lower($1) OR lower(new_email) = lower($1)) +
			(SELECT count(*) FROM pending_provider_links WHERE lower(coalesce(provider_email, '')) = lower($1)) +
			(SELECT count(*) FROM admin_audit_events WHERE lower(target_email) = lower($1))
	`, email).Scan(&count)
	if err != nil {
		t.Fatalf("count email references failed: %v", err)
	}
	return count
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
		if got := testutil.CountEmailJobs(t, ctx, user.Email, domain.EmailTemplateAuthMethodRemoved); got != 1 {
			t.Fatalf("auth-method-removed email jobs = %d, want 1", got)
		}
		data := testutil.LatestEmailTemplateData(t, ctx, user.Email, domain.EmailTemplateAuthMethodRemoved)
		if data[email.TemplateVariableAuthMethod] != string(domain.ProviderPassword) {
			t.Fatalf("unexpected auth-method-removed template data: %#v", data)
		}
	})
}

func TestAddAndChangePasswordQueueSecurityNotifications(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "password-notifications@example.com",
			Username: "password-notifications",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}
		googleID := "password-notifications-google"
		if _, err := tdb.Store.CreateAuthProvider(ctx, domain.AuthProvider{
			UserID:         user.ID,
			Provider:       domain.ProviderGoogle,
			ProviderUserID: &googleID,
		}); err != nil {
			t.Fatalf("CreateAuthProvider failed: %v", err)
		}
		currentHash, err := Hash("current-password")
		if err != nil {
			t.Fatalf("Hash current password failed: %v", err)
		}
		svc := New(Config{Store: tdb.Store, Tx: tdb.Tx})
		if err := svc.AddPassword(ctx, user.ID, currentHash); err != nil {
			t.Fatalf("AddPassword failed: %v", err)
		}
		if got := testutil.CountEmailJobs(t, ctx, user.Email, domain.EmailTemplateAuthMethodAdded); got != 1 {
			t.Fatalf("auth-method-added email jobs = %d, want 1", got)
		}
		addedData := testutil.LatestEmailTemplateData(t, ctx, user.Email, domain.EmailTemplateAuthMethodAdded)
		if addedData[email.TemplateVariableAuthMethod] != string(domain.ProviderPassword) {
			t.Fatalf("unexpected auth-method-added template data: %#v", addedData)
		}

		newHash, err := Hash("new-password")
		if err != nil {
			t.Fatalf("Hash new password failed: %v", err)
		}
		organization, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username)
		if err != nil {
			t.Fatalf("EnsureDefaultOrganizationForUser failed: %v", err)
		}
		session, err := tdb.Store.CreateSession(ctx, domain.Session{
			UserID:               user.ID,
			ActiveOrganizationID: organization.ID,
			ExpiresAt:            time.Now().UTC().Add(time.Hour),
			UserAgent:            "password-change-test",
		})
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}
		if err := svc.ChangePassword(ctx, user.ID, session.ID, "current-password", newHash); err != nil {
			t.Fatalf("ChangePassword failed: %v", err)
		}
		updatedSession, err := tdb.Store.GetSessionByID(ctx, session.ID)
		if err != nil {
			t.Fatalf("GetSessionByID failed: %v", err)
		}
		if updatedSession.AuthenticatedAt == nil || updatedSession.AuthenticationMethod != domain.AuthenticationMethodPassword {
			t.Fatalf("password change did not update session authentication: %+v", updatedSession)
		}
		if got := testutil.CountEmailJobs(t, ctx, user.Email, domain.EmailTemplatePasswordChanged); got != 1 {
			t.Fatalf("password-changed email jobs = %d, want 1", got)
		}
	})
}

func TestChangePasswordRevokesOtherSessionFamiliesAndPendingReset(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Now().UTC()
		currentHash, err := Hash("current-password")
		if err != nil {
			t.Fatalf("Hash current password failed: %v", err)
		}
		user := createPasswordUser(
			t,
			ctx,
			tdb,
			"password-change-revocation@example.com",
			"password-change-revocation",
			currentHash,
		)
		organization, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username)
		if err != nil {
			t.Fatalf("EnsureDefaultOrganizationForUser failed: %v", err)
		}

		sessions := make([]domain.Session, 0, 3)
		refreshTokenHashes := make(map[uuid.UUID]string, 3)
		for _, userAgent := range []string{"current", "other-a", "other-b"} {
			session, err := tdb.Store.CreateSession(ctx, domain.Session{
				UserID:               user.ID,
				ActiveOrganizationID: organization.ID,
				ExpiresAt:            now.Add(time.Hour),
				UserAgent:            userAgent,
			})
			if err != nil {
				t.Fatalf("CreateSession(%s) failed: %v", userAgent, err)
			}
			refreshTokenHash := "refresh-" + userAgent
			if err := tdb.Store.CreateRefreshToken(ctx, domain.RefreshToken{
				SessionID:      session.ID,
				OrganizationID: organization.ID,
				TokenHash:      refreshTokenHash,
				ExpiresAt:      now.Add(time.Hour),
			}); err != nil {
				t.Fatalf("CreateRefreshToken(%s) failed: %v", userAgent, err)
			}
			sessions = append(sessions, session)
			refreshTokenHashes[session.ID] = refreshTokenHash
		}
		currentSession := sessions[0]

		resetChallenge, err := tdb.Store.CreateChallenge(ctx, domain.Challenge{
			Purpose:      domain.ChallengePurposePasswordReset,
			Email:        user.Email,
			ExpiresAt:    now.Add(30 * time.Minute),
			MaxAttempts:  5,
			MaxResends:   3,
			AttemptCount: 0,
			ResendCount:  0,
		})
		if err != nil {
			t.Fatalf("CreateChallenge failed: %v", err)
		}
		verification := challenge.NewVerificationCodeService(
			tdb.Store,
			10*time.Minute,
			[]byte("01234567890123456789012345678901"),
		)
		resetCode, err := verification.GenerateCode(ctx, resetChallenge, now)
		if err != nil {
			t.Fatalf("GenerateCode failed: %v", err)
		}
		if _, err := tdb.Store.CreatePendingPasswordReset(ctx, domain.PendingPasswordReset{
			ChallengeID:  resetChallenge.ID,
			UserID:       user.ID,
			PasswordHash: "reset-password-hash",
		}); err != nil {
			t.Fatalf("CreatePendingPasswordReset failed: %v", err)
		}

		cacheStore := &authServiceTestCache{values: make(map[string][]byte)}
		revocations := token.NewAccessTokenRevocations(cacheStore, time.Hour)
		svc := New(Config{
			Store:                  tdb.Store,
			Tx:                     tdb.Tx,
			AccessTokenRevocations: revocations,
		})
		newHash, err := Hash("new-password")
		if err != nil {
			t.Fatalf("Hash new password failed: %v", err)
		}
		if err := svc.ChangePassword(ctx, user.ID, currentSession.ID, "current-password", newHash); err != nil {
			t.Fatalf("ChangePassword failed: %v", err)
		}

		current, err := tdb.Store.GetSessionByID(ctx, currentSession.ID)
		if err != nil {
			t.Fatalf("GetSessionByID(current) failed: %v", err)
		}
		if current.RevokedAt != nil {
			t.Fatalf("current session was revoked at %v", current.RevokedAt)
		}
		if current.AuthenticatedAt == nil || current.AuthenticationMethod != domain.AuthenticationMethodPassword {
			t.Fatalf("current session authentication was not refreshed: %+v", current)
		}
		if _, err := tdb.Store.GetRefreshTokenByHash(ctx, refreshTokenHashes[current.ID]); err != nil {
			t.Fatalf("current refresh token was removed: %v", err)
		}

		claimsForSession := func(sessionID uuid.UUID) *token.AccessClaims {
			return &token.AccessClaims{
				SessionID: sessionID,
				OrgID:     organization.ID,
				RegisteredClaims: jwt.RegisteredClaims{
					Subject:  user.ID.String(),
					IssuedAt: jwt.NewNumericDate(now.Add(-time.Minute)),
				},
			}
		}
		if err := revocations.Check(ctx, "current-access-token", claimsForSession(current.ID)); err != nil {
			t.Fatalf("current access token was revoked: %v", err)
		}

		for _, other := range sessions[1:] {
			stored, err := tdb.Store.GetSessionByID(ctx, other.ID)
			if err != nil {
				t.Fatalf("GetSessionByID(%s) failed: %v", other.UserAgent, err)
			}
			if stored.RevokedAt == nil {
				t.Fatalf("other session %s remains active", other.UserAgent)
			}
			if _, err := tdb.Store.GetRefreshTokenByHash(ctx, refreshTokenHashes[other.ID]); !errors.Is(err, store.ErrRefreshTokenNotFound) {
				t.Fatalf("other refresh token %s remains usable: %v", other.UserAgent, err)
			}
			if err := revocations.Check(ctx, "access-token-"+other.UserAgent, claimsForSession(other.ID)); !errors.Is(err, token.ErrRevokedToken) {
				t.Fatalf("other access token %s revocation error = %v, want %v", other.UserAgent, err, token.ErrRevokedToken)
			}
		}

		if _, err := tdb.Store.GetPendingPasswordResetByChallengeID(ctx, resetChallenge.ID); !errors.Is(err, store.ErrorPendingPasswordResetNotFound) {
			t.Fatalf("pending password reset survived password change: %v", err)
		}
		resetService := challenge.New(challenge.Config{
			Store:        tdb.Store,
			Tx:           tdb.Tx,
			ChallengeTTL: 30 * time.Minute,
			MaxAttempts:  5,
			MaxResends:   3,
		})
		if err := resetService.CompletePasswordResetChallenge(ctx, resetChallenge.ID, resetCode, verification, now.Add(time.Minute)); !errors.Is(err, challenge.ErrPasswordResetUnavailable) {
			t.Fatalf("old password-reset code completion error = %v, want %v", err, challenge.ErrPasswordResetUnavailable)
		}

		provider, err := tdb.Store.GetAuthProviderByMethodAndUserID(ctx, domain.ProviderPassword, user.ID)
		if err != nil {
			t.Fatalf("GetAuthProviderByMethodAndUserID failed: %v", err)
		}
		valid, err := Verify("new-password", *provider.PasswordHash)
		if err != nil || !valid {
			t.Fatalf("new password does not verify, valid=%t err=%v", valid, err)
		}
	})
}

func TestChangePasswordRollsBackWhenSessionAuthenticationCannotBeUpdated(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		currentHash, err := Hash("current-password")
		if err != nil {
			t.Fatalf("Hash current password failed: %v", err)
		}
		user := createPasswordUser(
			t,
			ctx,
			tdb,
			"password-change-rollback@example.com",
			"password-change-rollback",
			currentHash,
		)
		newHash, err := Hash("new-password")
		if err != nil {
			t.Fatalf("Hash new password failed: %v", err)
		}

		svc := New(Config{Store: tdb.Store, Tx: tdb.Tx})
		err = svc.ChangePassword(ctx, user.ID, uuid.New(), "current-password", newHash)
		if !errors.Is(err, store.ErrSessionNotFound) {
			t.Fatalf("ChangePassword error = %v, want %v", err, store.ErrSessionNotFound)
		}

		provider, err := tdb.Store.GetAuthProviderByMethodAndUserID(ctx, domain.ProviderPassword, user.ID)
		if err != nil {
			t.Fatalf("GetAuthProviderByMethodAndUserID failed: %v", err)
		}
		valid, err := Verify("current-password", *provider.PasswordHash)
		if err != nil || !valid {
			t.Fatalf("original password was not preserved, valid=%t err=%v", valid, err)
		}
		valid, err = Verify("new-password", *provider.PasswordHash)
		if err != nil {
			t.Fatalf("Verify new password failed: %v", err)
		}
		if valid {
			t.Fatal("new password persisted despite failed session authentication update")
		}
		if got := testutil.CountEmailJobs(t, ctx, user.Email, domain.EmailTemplatePasswordChanged); got != 0 {
			t.Fatalf("password-changed email jobs = %d, want 0", got)
		}
	})
}

func TestChangePasswordRollsBackWhenSecurityEventCannotBePersisted(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		currentHash, err := Hash("current-password")
		if err != nil {
			t.Fatal(err)
		}
		user := createPasswordUser(t, ctx, tdb, "password-event-rollback@example.com", "password-event-rollback", currentHash)
		organization, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username)
		if err != nil {
			t.Fatal(err)
		}
		session, err := tdb.Store.CreateSession(ctx, domain.Session{
			UserID: user.ID, ActiveOrganizationID: organization.ID,
			ExpiresAt: time.Now().UTC().Add(time.Hour), UserAgent: "password-event-rollback",
		})
		if err != nil {
			t.Fatal(err)
		}
		newHash, err := Hash("new-password")
		if err != nil {
			t.Fatal(err)
		}
		eventErr := errors.New("security event unavailable")
		svc := New(Config{
			Store: tdb.Store, Tx: tdb.Tx,
			SecurityEvents: failingPasswordChangedRecorder{err: eventErr},
		})
		if err := svc.ChangePassword(ctx, user.ID, session.ID, "current-password", newHash); !errors.Is(err, eventErr) {
			t.Fatalf("ChangePassword error = %v, want %v", err, eventErr)
		}

		provider, err := tdb.Store.GetAuthProviderByMethodAndUserID(ctx, domain.ProviderPassword, user.ID)
		if err != nil {
			t.Fatal(err)
		}
		valid, err := Verify("current-password", *provider.PasswordHash)
		if err != nil || !valid {
			t.Fatalf("original password was not preserved, valid=%t err=%v", valid, err)
		}
		persistedSession, err := tdb.Store.GetSessionByID(ctx, session.ID)
		if err != nil {
			t.Fatal(err)
		}
		if persistedSession.AuthenticatedAt != nil || persistedSession.AuthenticationMethod != "" {
			t.Fatalf("session authentication survived event failure: %+v", persistedSession)
		}
		if got := testutil.CountEmailJobs(t, ctx, user.Email, domain.EmailTemplatePasswordChanged); got != 0 {
			t.Fatalf("password-changed email jobs = %d, want 0", got)
		}
	})
}

func TestChangePasswordRollsBackDatabaseWhenAccessTokenRevocationFails(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Now().UTC()
		currentHash, err := Hash("current-password")
		if err != nil {
			t.Fatalf("Hash current password failed: %v", err)
		}
		user := createPasswordUser(
			t,
			ctx,
			tdb,
			"password-change-cache-failure@example.com",
			"password-change-cache-failure",
			currentHash,
		)
		organization, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username)
		if err != nil {
			t.Fatalf("EnsureDefaultOrganizationForUser failed: %v", err)
		}
		currentSession, err := tdb.Store.CreateSession(ctx, domain.Session{
			UserID:               user.ID,
			ActiveOrganizationID: organization.ID,
			ExpiresAt:            now.Add(time.Hour),
			UserAgent:            "current",
		})
		if err != nil {
			t.Fatalf("CreateSession(current) failed: %v", err)
		}
		otherSessions := make([]domain.Session, 0, 2)
		otherRefreshTokenHashes := make(map[uuid.UUID]string, 2)
		for _, userAgent := range []string{"other-a", "other-b"} {
			otherSession, err := tdb.Store.CreateSession(ctx, domain.Session{
				UserID:               user.ID,
				ActiveOrganizationID: organization.ID,
				ExpiresAt:            now.Add(time.Hour),
				UserAgent:            userAgent,
			})
			if err != nil {
				t.Fatalf("CreateSession(%s) failed: %v", userAgent, err)
			}
			refreshTokenHash := "cache-failure-refresh-" + userAgent
			if err := tdb.Store.CreateRefreshToken(ctx, domain.RefreshToken{
				SessionID:      otherSession.ID,
				OrganizationID: organization.ID,
				TokenHash:      refreshTokenHash,
				ExpiresAt:      now.Add(time.Hour),
			}); err != nil {
				t.Fatalf("CreateRefreshToken(%s) failed: %v", userAgent, err)
			}
			otherSessions = append(otherSessions, otherSession)
			otherRefreshTokenHashes[otherSession.ID] = refreshTokenHash
		}
		resetChallenge, err := tdb.Store.CreateChallenge(ctx, domain.Challenge{
			Purpose:     domain.ChallengePurposePasswordReset,
			Email:       user.Email,
			ExpiresAt:   now.Add(30 * time.Minute),
			MaxAttempts: 5,
			MaxResends:  3,
		})
		if err != nil {
			t.Fatalf("CreateChallenge failed: %v", err)
		}
		if _, err := tdb.Store.CreatePendingPasswordReset(ctx, domain.PendingPasswordReset{
			ChallengeID:  resetChallenge.ID,
			UserID:       user.ID,
			PasswordHash: "reset-password-hash",
		}); err != nil {
			t.Fatalf("CreatePendingPasswordReset failed: %v", err)
		}

		cacheErr := errors.New("revocation cache unavailable")
		cacheStore := &authServiceTestCache{
			values:    make(map[string][]byte),
			setErr:    cacheErr,
			failAfter: 1,
		}
		revocations := token.NewAccessTokenRevocations(cacheStore, time.Hour)
		svc := New(Config{
			Store:                  tdb.Store,
			Tx:                     tdb.Tx,
			AccessTokenRevocations: revocations,
		})
		newHash, err := Hash("new-password")
		if err != nil {
			t.Fatalf("Hash new password failed: %v", err)
		}
		if err := svc.ChangePassword(ctx, user.ID, currentSession.ID, "current-password", newHash); !errors.Is(err, cacheErr) {
			t.Fatalf("ChangePassword error = %v, want %v", err, cacheErr)
		}

		provider, err := tdb.Store.GetAuthProviderByMethodAndUserID(ctx, domain.ProviderPassword, user.ID)
		if err != nil {
			t.Fatalf("GetAuthProviderByMethodAndUserID failed: %v", err)
		}
		valid, err := Verify("current-password", *provider.PasswordHash)
		if err != nil || !valid {
			t.Fatalf("original password was not preserved, valid=%t err=%v", valid, err)
		}
		current, err := tdb.Store.GetSessionByID(ctx, currentSession.ID)
		if err != nil {
			t.Fatalf("GetSessionByID(current) failed: %v", err)
		}
		if current.AuthenticatedAt != nil || current.AuthenticationMethod != "" {
			t.Fatalf("current session authentication changed despite failure: %+v", current)
		}
		for _, otherSession := range otherSessions {
			other, err := tdb.Store.GetSessionByID(ctx, otherSession.ID)
			if err != nil {
				t.Fatalf("GetSessionByID(%s) failed: %v", otherSession.UserAgent, err)
			}
			if other.RevokedAt != nil {
				t.Fatalf("other session %s was revoked in the database despite failure: %+v", otherSession.UserAgent, other)
			}
			if _, err := tdb.Store.GetRefreshTokenByHash(ctx, otherRefreshTokenHashes[otherSession.ID]); err != nil {
				t.Fatalf("other refresh token %s was removed despite failure: %v", otherSession.UserAgent, err)
			}
		}
		if _, err := tdb.Store.GetPendingPasswordResetByChallengeID(ctx, resetChallenge.ID); err != nil {
			t.Fatalf("pending password reset was removed despite failure: %v", err)
		}
		if got := testutil.CountEmailJobs(t, ctx, user.Email, domain.EmailTemplatePasswordChanged); got != 0 {
			t.Fatalf("password-changed email jobs = %d, want 0", got)
		}

		claimsForSession := func(sessionID uuid.UUID) *token.AccessClaims {
			return &token.AccessClaims{
				SessionID: sessionID,
				OrgID:     organization.ID,
				RegisteredClaims: jwt.RegisteredClaims{
					Subject:  user.ID.String(),
					IssuedAt: jwt.NewNumericDate(now.Add(-time.Minute)),
				},
			}
		}
		revokedCount := 0
		for i, otherSession := range otherSessions {
			err := revocations.Check(ctx, "other-token", claimsForSession(otherSession.ID))
			switch {
			case errors.Is(err, token.ErrRevokedToken):
				revokedCount++
			case err != nil:
				t.Fatalf("check other access token %d failed: %v", i, err)
			}
		}
		if revokedCount != 1 {
			t.Fatalf("fail-closed access-token revocations = %d, want 1", revokedCount)
		}
	})
}

func TestCompleteProviderLinkQueuesSecurityNotification(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		passwordHash, err := Hash("current-password")
		if err != nil {
			t.Fatalf("Hash failed: %v", err)
		}
		user := createPasswordUser(t, ctx, tdb, "provider-link-notification@example.com", "provider-link-notification", passwordHash)
		svc := New(Config{
			Store: tdb.Store,
			Tx:    tdb.Tx,
			OAuthProviders: oauth.OAuthProviders{Providers: []oauth.OAuthProvider{
				oauth.NewOAuthProvider(domain.ProviderGoogle, "client-id", "http://localhost:3000"),
			}},
		})
		now := time.Date(2026, 9, 9, 18, 30, 0, 0, time.UTC)
		organization, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username)
		if err != nil {
			t.Fatalf("EnsureDefaultOrganizationForUser failed: %v", err)
		}
		session, err := tdb.Store.CreateSession(ctx, domain.Session{
			UserID:               user.ID,
			ActiveOrganizationID: organization.ID,
			ExpiresAt:            now.Add(time.Hour),
			UserAgent:            "provider-link-test",
		})
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}
		sessionID := session.ID
		linkID, err := svc.StartProviderLink(ctx, user.ID, sessionID, domain.ProviderGoogle, now)
		if err != nil {
			t.Fatalf("StartProviderLink failed: %v", err)
		}
		if err := svc.CompleteProviderLink(
			ctx,
			linkID,
			user.ID,
			sessionID,
			domain.ProviderGoogle,
			"provider-link-google-id",
			user.Email,
			true,
			now,
		); err != nil {
			t.Fatalf("CompleteProviderLink failed: %v", err)
		}
		if got := testutil.CountEmailJobs(t, ctx, user.Email, domain.EmailTemplateAuthMethodAdded); got != 1 {
			t.Fatalf("auth-method-added email jobs = %d, want 1", got)
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
		passwordHash, err := Hash("correct-password")
		if err != nil {
			t.Fatalf("Hash failed: %v", err)
		}
		createPasswordUser(t, ctx, tdb, "login-policy-error@example.com", "login-policy-error", passwordHash)

		svc := New(Config{
			Store:        tdb.Store,
			Tx:           tdb.Tx,
			AccessPolicy: staticAccessPolicy{err: policyErr},
		})

		_, err = svc.Login(ctx, LoginInput{
			Provider: domain.ProviderPassword,
			Email:    "login-policy-error@example.com",
			Password: "correct-password",
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
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("Login error = %v, want ErrInvalidCredentials", err)
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
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("Login error = %v, want ErrInvalidCredentials", err)
		}
	})
}

func TestLogin_NullPasswordHashUsesDummy(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "null-password-hash@example.com",
			Username: "null-password-hash",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}
		if _, err := tdb.Store.CreateAuthProvider(ctx, domain.AuthProvider{
			UserID:   user.ID,
			Provider: domain.ProviderPassword,
		}); err != nil {
			t.Fatalf("CreateAuthProvider failed: %v", err)
		}

		svc := New(Config{Store: tdb.Store, Tx: tdb.Tx, AccessPolicy: staticAccessPolicy{allowed: true}})
		_, err = svc.Login(ctx, LoginInput{
			Provider: domain.ProviderPassword,
			Email:    user.Email,
			Password: "irrelevant",
		})
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("Login error = %v, want ErrInvalidCredentials", err)
		}
	})
}

func TestLogin_InvalidCredentialPathsHaveComparableTiming(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		passwordHash, err := Hash("correct-password")
		if err != nil {
			t.Fatalf("Hash failed: %v", err)
		}
		createPasswordUser(t, ctx, tdb, "timing-password@example.com", "timing-password", passwordHash)
		if _, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "timing-oauth-only@example.com",
			Username: "timing-oauth-only",
		}); err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}

		svc := New(Config{Store: tdb.Store, Tx: tdb.Tx, AccessPolicy: staticAccessPolicy{allowed: true}})
		cases := []struct {
			name  string
			email string
		}{
			{name: "known-wrong", email: "timing-password@example.com"},
			{name: "unknown-user", email: "timing-missing@example.com"},
			{name: "missing-provider", email: "timing-oauth-only@example.com"},
		}
		durations := make([][]time.Duration, len(cases))

		const samples = 5
		for sample := 0; sample < samples; sample++ {
			for offset := range cases {
				index := (sample + offset) % len(cases)
				testCase := cases[index]
				started := time.Now()
				_, err := svc.Login(ctx, LoginInput{
					Provider: domain.ProviderPassword,
					Email:    testCase.email,
					Password: "wrong-password",
				})
				durations[index] = append(durations[index], time.Since(started))
				if !errors.Is(err, ErrInvalidCredentials) {
					t.Fatalf("%s Login error = %v, want ErrInvalidCredentials", testCase.name, err)
				}
			}
		}

		medians := make([]time.Duration, len(cases))
		for index := range cases {
			slices.Sort(durations[index])
			medians[index] = durations[index][samples/2]
		}
		minimum := slices.Min(medians)
		maximum := slices.Max(medians)
		if maximum > 3*minimum {
			t.Fatalf("login timing medians differ by more than 3x: known-wrong=%s unknown-user=%s missing-provider=%s", medians[0], medians[1], medians[2])
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
		if got := testutil.CountEmailJobs(t, ctx, user.Email, domain.EmailTemplateAuthMethodAdded); got != 1 {
			t.Fatalf("recovery auth-method-added email jobs = %d, want 1", got)
		}
		data := testutil.LatestEmailTemplateData(t, ctx, user.Email, domain.EmailTemplateAuthMethodAdded)
		if data[email.TemplateVariableAuthMethod] != string(domain.ProviderGoogle) {
			t.Fatalf("unexpected recovery auth-method-added template data: %#v", data)
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
