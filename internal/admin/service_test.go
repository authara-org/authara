package admin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/session/roles"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/google/uuid"
)

func TestDisableUserRejectsSelf(t *testing.T) {
	id := uuid.New()
	svc := New(Config{})

	err := svc.DisableUser(context.Background(), Actor{UserID: id}, id, RequestMeta{})
	if !errors.Is(err, ErrSelfDisable) {
		t.Fatalf("expected ErrSelfDisable, got %v", err)
	}
}

func TestDisableUserRejectsLastActiveAdmin(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		target := createAdminTestUser(t, ctx, tdb, "last-disable@example.com", "last-disable", true)
		svc := newAdminTestService(tdb)

		err := svc.DisableUser(ctx, Actor{UserID: uuid.New()}, target.ID, RequestMeta{})
		if !errors.Is(err, ErrLastAdmin) {
			t.Fatalf("expected ErrLastAdmin, got %v", err)
		}
	})
}

func TestDisableUserRevokesSessionsAndAudits(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		actor := createAdminTestUser(t, ctx, tdb, "disable-actor@example.com", "disable-actor", true)
		target := createAdminTestUser(t, ctx, tdb, "disable-target@example.com", "disable-target", false)
		session := createAdminTestSession(t, ctx, tdb, target.ID)
		if err := tdb.Store.CreateRefreshToken(ctx, domain.RefreshToken{
			SessionID:      session.ID,
			OrganizationID: session.ActiveOrganizationID,
			TokenHash:      "disable-refresh-hash",
			ExpiresAt:      fixedAdminTestNow().Add(time.Hour),
		}); err != nil {
			t.Fatalf("CreateRefreshToken failed: %v", err)
		}

		svc := newAdminTestService(tdb)
		if err := svc.DisableUser(ctx, Actor{UserID: actor.ID}, target.ID, RequestMeta{IP: "127.0.0.1", UserAgent: "test-agent"}); err != nil {
			t.Fatalf("DisableUser failed: %v", err)
		}

		disabled, err := tdb.Store.IsUserDisabled(ctx, target.ID)
		if err != nil {
			t.Fatalf("IsUserDisabled failed: %v", err)
		}
		if !disabled {
			t.Fatal("expected target user to be disabled")
		}

		updatedSession, err := tdb.Store.GetSessionByID(ctx, session.ID)
		if err != nil {
			t.Fatalf("GetSessionByID failed: %v", err)
		}
		if updatedSession.RevokedAt == nil {
			t.Fatal("expected session to be revoked")
		}

		_, err = tdb.Store.GetRefreshTokenByHash(ctx, "disable-refresh-hash")
		if !errors.Is(err, store.ErrRefreshTokenNotFound) {
			t.Fatalf("expected refresh token to be deleted, got %v", err)
		}

		events, err := tdb.Store.ListAdminAuditEvents(ctx, store.AdminAuditEventFilter{Action: ActionUserDisabled})
		if err != nil {
			t.Fatalf("ListAdminAuditEvents failed: %v", err)
		}
		if len(events) != 1 {
			t.Fatalf("expected 1 audit event, got %d", len(events))
		}
		if events[0].ActorUserID == nil || *events[0].ActorUserID != actor.ID {
			t.Fatal("expected actor user id in audit event")
		}
	})
}

func TestDashboardStatsIncludesSummaryCounts(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Now().UTC()
		user := createAdminTestUser(t, ctx, tdb, "dashboard-user@example.com", "dashboard-user", false)
		disabled := createAdminTestUser(t, ctx, tdb, "dashboard-disabled@example.com", "dashboard-disabled", false)
		if err := tdb.Store.DisableUser(ctx, disabled.ID, now); err != nil {
			t.Fatalf("DisableUser failed: %v", err)
		}
		org, _, err := tdb.Store.GetPersonalOrganizationForUser(ctx, user.ID)
		if err != nil {
			t.Fatalf("GetPersonalOrganizationForUser failed: %v", err)
		}
		if _, err := tdb.Store.CreateSession(ctx, domain.Session{
			UserID:               user.ID,
			ActiveOrganizationID: org.ID,
			ExpiresAt:            now.Add(time.Hour),
			UserAgent:            "dashboard-test",
		}); err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}

		svc := New(Config{
			Store: tdb.Store,
			Tx:    tdb.Tx,
			Now: func() time.Time {
				return now
			},
		})
		stats, err := svc.DashboardStats(ctx)
		if err != nil {
			t.Fatalf("DashboardStats failed: %v", err)
		}
		if stats.TotalUsers < 2 {
			t.Fatalf("expected at least 2 total users, got %d", stats.TotalUsers)
		}
		if stats.SignupsLast24Hours < 2 {
			t.Fatalf("expected at least 2 signups in last 24 hours, got %d", stats.SignupsLast24Hours)
		}
		if stats.DisabledUsers < 1 {
			t.Fatalf("expected at least 1 disabled user, got %d", stats.DisabledUsers)
		}
		if stats.ActiveSessions < 1 {
			t.Fatalf("expected at least 1 active session, got %d", stats.ActiveSessions)
		}
	})
}

func TestSearchUserFindsEmailOrUsername(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user := createAdminTestUser(t, ctx, tdb, "service-search-user@example.com", "ServiceSearchUser", false)
		svc := newAdminTestService(tdb)

		byEmail, err := svc.SearchUser(ctx, " service-search-user@example.com ")
		if err != nil {
			t.Fatalf("SearchUser by email failed: %v", err)
		}
		if byEmail.ID != user.ID {
			t.Fatalf("expected email lookup user %s, got %s", user.ID, byEmail.ID)
		}

		byUsername, err := svc.SearchUser(ctx, "servicesearchuser")
		if err != nil {
			t.Fatalf("SearchUser by username failed: %v", err)
		}
		if byUsername.ID != user.ID {
			t.Fatalf("expected username lookup user %s, got %s", user.ID, byUsername.ID)
		}
	})
}

func TestEnableUser(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		actor := createAdminTestUser(t, ctx, tdb, "enable-actor@example.com", "enable-actor", true)
		target := createAdminTestUser(t, ctx, tdb, "enable-target@example.com", "enable-target", false)
		if err := tdb.Store.DisableUser(ctx, target.ID, fixedAdminTestNow()); err != nil {
			t.Fatalf("DisableUser setup failed: %v", err)
		}

		svc := newAdminTestService(tdb)
		if err := svc.EnableUser(ctx, Actor{UserID: actor.ID}, target.ID, RequestMeta{}); err != nil {
			t.Fatalf("EnableUser failed: %v", err)
		}

		disabled, err := tdb.Store.IsUserDisabled(ctx, target.ID)
		if err != nil {
			t.Fatalf("IsUserDisabled failed: %v", err)
		}
		if disabled {
			t.Fatal("expected target user to be enabled")
		}
	})
}

func TestGrantAndRevokeAdmin(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		actor := createAdminTestUser(t, ctx, tdb, "grant-actor@example.com", "grant-actor", true)
		target := createAdminTestUser(t, ctx, tdb, "grant-target@example.com", "grant-target", false)
		createAdminTestSession(t, ctx, tdb, target.ID)

		svc := newAdminTestService(tdb)
		if err := svc.GrantAdmin(ctx, Actor{UserID: actor.ID}, target.ID, RequestMeta{}); err != nil {
			t.Fatalf("GrantAdmin failed: %v", err)
		}
		hasAdmin, err := tdb.Store.UserHasPlatformRole(ctx, target.ID, roles.DBAdminRoleName)
		if err != nil {
			t.Fatalf("UserHasPlatformRole failed: %v", err)
		}
		if !hasAdmin {
			t.Fatal("expected target to have admin role")
		}

		if err := svc.RevokeAdmin(ctx, Actor{UserID: actor.ID}, target.ID, RequestMeta{}); err != nil {
			t.Fatalf("RevokeAdmin failed: %v", err)
		}
		hasAdmin, err = tdb.Store.UserHasPlatformRole(ctx, target.ID, roles.DBAdminRoleName)
		if err != nil {
			t.Fatalf("UserHasPlatformRole after revoke failed: %v", err)
		}
		if hasAdmin {
			t.Fatal("expected target admin role to be removed")
		}
		activeSessions, err := tdb.Store.CountActiveSessionsByUserID(ctx, target.ID, fixedAdminTestNow())
		if err != nil {
			t.Fatalf("CountActiveSessionsByUserID failed: %v", err)
		}
		if activeSessions != 0 {
			t.Fatalf("expected sessions to be revoked after admin revoke, got %d", activeSessions)
		}
	})
}

func TestRevokeAdminRejectsSelfAndLastAdmin(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		target := createAdminTestUser(t, ctx, tdb, "last-revoke@example.com", "last-revoke", true)
		svc := newAdminTestService(tdb)

		err := svc.RevokeAdmin(ctx, Actor{UserID: target.ID}, target.ID, RequestMeta{})
		if !errors.Is(err, ErrSelfRevokeAdmin) {
			t.Fatalf("expected ErrSelfRevokeAdmin, got %v", err)
		}

		err = svc.RevokeAdmin(ctx, Actor{UserID: uuid.New()}, target.ID, RequestMeta{})
		if !errors.Is(err, ErrLastAdmin) {
			t.Fatalf("expected ErrLastAdmin, got %v", err)
		}
	})
}

func TestRevokeAllUserSessionsRejectsSelf(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		actor := createAdminTestUser(t, ctx, tdb, "revoke-self-sessions@example.com", "revoke-self-sessions", true)
		svc := newAdminTestService(tdb)

		err := svc.RevokeAllUserSessions(ctx, Actor{UserID: actor.ID}, actor.ID, RequestMeta{})
		if !errors.Is(err, ErrSelfRevokeSessions) {
			t.Fatalf("expected ErrSelfRevokeSessions, got %v", err)
		}
	})
}

func TestGetUserDetailActionAvailability(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		actor := createAdminTestUser(t, ctx, tdb, "detail-actor@example.com", "detail-actor", true)
		target := createAdminTestUser(t, ctx, tdb, "detail-target@example.com", "detail-target", true)
		svc := newAdminTestService(tdb)

		selfDetail, err := svc.GetUserDetail(ctx, Actor{UserID: actor.ID}, actor.ID)
		if err != nil {
			t.Fatalf("GetUserDetail self failed: %v", err)
		}
		if selfDetail.Actions.Disable.Allowed || selfDetail.Actions.Disable.Reason != ReasonSelfDisable {
			t.Fatalf("expected self disable to be blocked, got %+v", selfDetail.Actions.Disable)
		}
		if selfDetail.Actions.RevokeAdmin.Allowed || selfDetail.Actions.RevokeAdmin.Reason != ReasonSelfRevokeAdmin {
			t.Fatalf("expected self admin revoke to be blocked, got %+v", selfDetail.Actions.RevokeAdmin)
		}
		if selfDetail.Actions.RevokeAllSessions.Allowed || selfDetail.Actions.RevokeAllSessions.Reason != ReasonSelfRevokeSessions {
			t.Fatalf("expected self revoke sessions to be blocked, got %+v", selfDetail.Actions.RevokeAllSessions)
		}

		targetDetail, err := svc.GetUserDetail(ctx, Actor{UserID: actor.ID}, target.ID)
		if err != nil {
			t.Fatalf("GetUserDetail target failed: %v", err)
		}
		if !targetDetail.Actions.Disable.Allowed {
			t.Fatalf("expected target disable to be allowed, got %+v", targetDetail.Actions.Disable)
		}
		if !targetDetail.Actions.RevokeAdmin.Allowed {
			t.Fatalf("expected target admin revoke to be allowed, got %+v", targetDetail.Actions.RevokeAdmin)
		}
		if !targetDetail.Actions.RevokeAllSessions.Allowed {
			t.Fatalf("expected target session revoke to be allowed, got %+v", targetDetail.Actions.RevokeAllSessions)
		}
	})
}

func TestAuditMetadataSanitizesSecrets(t *testing.T) {
	out := sanitizeAuditMetadata(map[string]any{
		"session_id":       "session-123",
		"password_hash":    "secret-hash",
		"request_body":     "raw-body",
		"refresh_token":    "secret-token",
		"verificationCode": "123456",
		"nested": map[string]any{
			"public_key": "secret-public-key",
			"safe":       "kept",
		},
	})

	if out["session_id"] != "session-123" {
		t.Fatalf("expected safe session_id to remain, got %+v", out)
	}
	for _, key := range []string{"password_hash", "request_body", "refresh_token", "verificationCode"} {
		if _, ok := out[key]; ok {
			t.Fatalf("expected %q to be removed from audit metadata: %+v", key, out)
		}
	}
	nested, ok := out["nested"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested map, got %+v", out["nested"])
	}
	if _, ok := nested["public_key"]; ok {
		t.Fatalf("expected nested public_key to be removed: %+v", nested)
	}
	if nested["safe"] != "kept" {
		t.Fatalf("expected nested safe value to remain: %+v", nested)
	}
}

func TestAllowlistAddNormalizesAndRemoveAudits(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		actor := createAdminTestUser(t, ctx, tdb, "allowlist-actor@example.com", "allowlist-actor", true)
		svc := newAdminTestService(tdb)

		if err := svc.AddAllowedEmail(ctx, Actor{UserID: actor.ID}, "  USER@EXAMPLE.COM  ", RequestMeta{}); err != nil {
			t.Fatalf("AddAllowedEmail failed: %v", err)
		}
		page, err := svc.ListAllowedEmails(ctx, "user@example.com", Page{Page: 1, Size: 10})
		if err != nil {
			t.Fatalf("ListAllowedEmails failed: %v", err)
		}
		if len(page.Emails) != 1 || page.Emails[0].Email != "user@example.com" {
			t.Fatalf("expected normalized email, got %+v", page.Emails)
		}

		if err := svc.RemoveAllowedEmail(ctx, Actor{UserID: actor.ID}, page.Emails[0].ID, RequestMeta{}); err != nil {
			t.Fatalf("RemoveAllowedEmail failed: %v", err)
		}
		events, err := tdb.Store.ListAdminAuditEvents(ctx, store.AdminAuditEventFilter{Action: ActionAllowlistEmailRemoved})
		if err != nil {
			t.Fatalf("ListAdminAuditEvents failed: %v", err)
		}
		if len(events) != 1 {
			t.Fatalf("expected allowlist removal audit event, got %d", len(events))
		}
	})
}

func TestListAllowedEmailsLiveSearchRules(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		actor := createAdminTestUser(t, ctx, tdb, "allowlist-search-actor@example.com", "allowlist-search-actor", true)
		svc := newAdminTestService(tdb)

		for _, email := range []string{"alpha-search@example.com", "beta-search@example.com", "gamma-search@example.com"} {
			if err := svc.AddAllowedEmail(ctx, Actor{UserID: actor.ID}, email, RequestMeta{}); err != nil {
				t.Fatalf("AddAllowedEmail(%s) failed: %v", email, err)
			}
		}

		all, err := svc.ListAllowedEmails(ctx, "", Page{Page: 1, Size: 2})
		if err != nil {
			t.Fatalf("ListAllowedEmails empty query failed: %v", err)
		}
		if len(all.Emails) != 2 || all.Total != 3 {
			t.Fatalf("expected paginated full allowlist, got %+v", all)
		}

		short, err := svc.ListAllowedEmails(ctx, "al", Page{Page: 1, Size: 25})
		if err != nil {
			t.Fatalf("ListAllowedEmails short query failed: %v", err)
		}
		if short.Message == "" || len(short.Emails) != 0 || short.Total != 0 {
			t.Fatalf("expected min-length message without results, got %+v", short)
		}

		filtered, err := svc.ListAllowedEmails(ctx, "ALPHA", Page{Page: 1, Size: 25})
		if err != nil {
			t.Fatalf("ListAllowedEmails filtered query failed: %v", err)
		}
		if len(filtered.Emails) != 1 || filtered.Emails[0].Email != "alpha-search@example.com" {
			t.Fatalf("expected case-insensitive filtered result, got %+v", filtered)
		}
	})
}

func TestAllowlistDuplicateHandledFriendly(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		actor := createAdminTestUser(t, ctx, tdb, "allowlist-duplicate-actor@example.com", "allowlist-duplicate-actor", true)
		svc := newAdminTestService(tdb)

		if err := svc.AddAllowedEmail(ctx, Actor{UserID: actor.ID}, "duplicate@example.com", RequestMeta{}); err != nil {
			t.Fatalf("AddAllowedEmail setup failed: %v", err)
		}

		err := svc.AddAllowedEmail(ctx, Actor{UserID: actor.ID}, "duplicate@example.com", RequestMeta{})
		if !errors.Is(err, ErrAllowedEmailAlreadyAdded) {
			t.Fatalf("expected ErrAllowedEmailAlreadyAdded, got %v", err)
		}
	})
}

func TestAllowlistMethodsRejectWhenDisabled(t *testing.T) {
	svc := New(Config{})
	ctx := context.Background()

	if _, err := svc.ListAllowedEmails(ctx, "", Page{Page: 1, Size: 10}); !errors.Is(err, ErrAllowlistDisabled) {
		t.Fatalf("expected ListAllowedEmails to return ErrAllowlistDisabled, got %v", err)
	}
	if err := svc.AddAllowedEmail(ctx, Actor{UserID: uuid.New()}, "user@example.com", RequestMeta{}); !errors.Is(err, ErrAllowlistDisabled) {
		t.Fatalf("expected AddAllowedEmail to return ErrAllowlistDisabled, got %v", err)
	}
	if err := svc.RemoveAllowedEmail(ctx, Actor{UserID: uuid.New()}, uuid.New(), RequestMeta{}); !errors.Is(err, ErrAllowlistDisabled) {
		t.Fatalf("expected RemoveAllowedEmail to return ErrAllowlistDisabled, got %v", err)
	}
}

func newAdminTestService(tdb *testutil.TestDB) *Service {
	return New(Config{
		Store:            tdb.Store,
		Tx:               tdb.Tx,
		Now:              fixedAdminTestNow,
		AllowlistEnabled: true,
	})
}

func createAdminTestUser(t *testing.T, ctx context.Context, tdb *testutil.TestDB, email, username string, adminRole bool) domain.User {
	t.Helper()

	user, err := tdb.Store.CreateUser(ctx, domain.User{
		Email:    email,
		Username: username,
	})
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	if _, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username); err != nil {
		t.Fatalf("EnsureDefaultOrganizationForUser failed: %v", err)
	}
	if adminRole {
		if err := tdb.Store.AddUserPlatformRoleByName(ctx, user.ID, roles.DBAdminRoleName); err != nil {
			t.Fatalf("AddUserPlatformRoleByName failed: %v", err)
		}
	}
	return user
}

func createAdminTestSession(t *testing.T, ctx context.Context, tdb *testutil.TestDB, userID uuid.UUID) domain.Session {
	t.Helper()

	org, _, err := tdb.Store.GetPersonalOrganizationForUser(ctx, userID)
	if err != nil {
		t.Fatalf("GetPersonalOrganizationForUser failed: %v", err)
	}
	session, err := tdb.Store.CreateSession(ctx, domain.Session{
		UserID:               userID,
		ActiveOrganizationID: org.ID,
		ExpiresAt:            fixedAdminTestNow().Add(time.Hour),
		UserAgent:            "admin-test",
	})
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	return session
}

func fixedAdminTestNow() time.Time {
	return time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
}
