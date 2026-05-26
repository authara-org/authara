package store_test

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

func TestAdminStoreCountsAndRoles(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		recentSince := time.Now().Add(-24 * time.Hour)
		recentBefore, err := tdb.Store.CountUsersCreatedSince(ctx, recentSince)
		if err != nil {
			t.Fatalf("CountUsersCreatedSince before setup failed: %v", err)
		}

		activeAdmin := createAdminStoreUser(t, ctx, tdb, "store-count-admin@example.com", "store-count-admin")
		disabledAdmin := createAdminStoreUser(t, ctx, tdb, "store-count-disabled@example.com", "store-count-disabled")
		user := createAdminStoreUser(t, ctx, tdb, "store-count-user@example.com", "store-count-user")

		if err := tdb.Store.AddUserPlatformRoleByName(ctx, activeAdmin.ID, roles.DBAdminRoleName); err != nil {
			t.Fatalf("AddUserPlatformRoleByName active failed: %v", err)
		}
		if err := tdb.Store.AddUserPlatformRoleByName(ctx, disabledAdmin.ID, roles.DBAdminRoleName); err != nil {
			t.Fatalf("AddUserPlatformRoleByName disabled failed: %v", err)
		}
		if err := tdb.Store.DisableUser(ctx, disabledAdmin.ID, adminStoreNow()); err != nil {
			t.Fatalf("DisableUser failed: %v", err)
		}

		total, err := tdb.Store.CountUsers(ctx)
		if err != nil {
			t.Fatalf("CountUsers failed: %v", err)
		}
		if total < 3 {
			t.Fatalf("expected at least 3 users, got %d", total)
		}

		recentAfter, err := tdb.Store.CountUsersCreatedSince(ctx, recentSince)
		if err != nil {
			t.Fatalf("CountUsersCreatedSince after setup failed: %v", err)
		}
		if recentAfter-recentBefore != 3 {
			t.Fatalf("expected 3 recent users, got %d", recentAfter-recentBefore)
		}

		admins, err := tdb.Store.CountUsersWithRole(ctx, roles.DBAdminRoleName)
		if err != nil {
			t.Fatalf("CountUsersWithRole failed: %v", err)
		}
		if admins != 2 {
			t.Fatalf("expected 2 admin users, got %d", admins)
		}

		activeAdmins, err := tdb.Store.CountActiveUsersWithRole(ctx, roles.DBAdminRoleName)
		if err != nil {
			t.Fatalf("CountActiveUsersWithRole failed: %v", err)
		}
		if activeAdmins != 1 {
			t.Fatalf("expected 1 active admin, got %d", activeAdmins)
		}

		hasAdmin, err := tdb.Store.UserHasPlatformRole(ctx, user.ID, roles.DBAdminRoleName)
		if err != nil {
			t.Fatalf("UserHasPlatformRole failed: %v", err)
		}
		if hasAdmin {
			t.Fatal("expected non-admin user not to have admin role")
		}
	})
}

func TestAdminStoreGetUserByEmailOrUsername(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user := createAdminStoreUser(t, ctx, tdb, "store-search-user@example.com", "StoreSearchUser")

		byEmail, err := tdb.Store.GetUserByEmailOrUsername(ctx, " store-search-user@example.com ")
		if err != nil {
			t.Fatalf("GetUserByEmailOrUsername by email failed: %v", err)
		}
		if byEmail.ID != user.ID {
			t.Fatalf("expected email lookup user %s, got %s", user.ID, byEmail.ID)
		}

		byUsername, err := tdb.Store.GetUserByEmailOrUsername(ctx, "storesearchuser")
		if err != nil {
			t.Fatalf("GetUserByEmailOrUsername by username failed: %v", err)
		}
		if byUsername.ID != user.ID {
			t.Fatalf("expected username lookup user %s, got %s", user.ID, byUsername.ID)
		}

		_, err = tdb.Store.GetUserByEmailOrUsername(ctx, "missing-store-search-user")
		if !errors.Is(err, store.ErrUserNotFound) {
			t.Fatalf("expected ErrUserNotFound, got %v", err)
		}
	})
}

func TestAdminStoreAllowedEmailsPaginationAndDelete(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		for _, email := range []string{"alpha@example.com", "beta@example.com", "Case@Example.com", "gamma@example.com"} {
			if err := tdb.Store.CreateAllowedEmail(ctx, domain.AllowedEmail{Email: email}); err != nil {
				t.Fatalf("CreateAllowedEmail(%s) failed: %v", email, err)
			}
		}

		count, err := tdb.Store.CountAllowedEmails(ctx, "alpha@")
		if err != nil {
			t.Fatalf("CountAllowedEmails failed: %v", err)
		}
		if count != 1 {
			t.Fatalf("expected one exact-like match, got %d", count)
		}

		caseCount, err := tdb.Store.CountAllowedEmails(ctx, "CASE@")
		if err != nil {
			t.Fatalf("CountAllowedEmails case-insensitive failed: %v", err)
		}
		if caseCount != 1 {
			t.Fatalf("expected one case-insensitive match, got %d", caseCount)
		}

		page, err := tdb.Store.ListAllowedEmailsPage(ctx, "example.com", 2, 0)
		if err != nil {
			t.Fatalf("ListAllowedEmailsPage failed: %v", err)
		}
		if len(page) != 2 {
			t.Fatalf("expected first page of 2 emails, got %d", len(page))
		}

		casePage, err := tdb.Store.ListAllowedEmailsPage(ctx, "case@", 10, 0)
		if err != nil {
			t.Fatalf("ListAllowedEmailsPage case-insensitive failed: %v", err)
		}
		if len(casePage) != 1 || casePage[0].Email != "Case@Example.com" {
			t.Fatalf("expected case-insensitive result, got %+v", casePage)
		}

		deleted, err := tdb.Store.DeleteAllowedEmailByID(ctx, page[0].ID)
		if err != nil {
			t.Fatalf("DeleteAllowedEmailByID failed: %v", err)
		}
		if deleted.Email != page[0].Email {
			t.Fatalf("expected deleted email %q, got %q", page[0].Email, deleted.Email)
		}

		_, err = tdb.Store.DeleteAllowedEmailByID(ctx, page[0].ID)
		if !errors.Is(err, store.ErrAllowedEmailNotFound) {
			t.Fatalf("expected ErrAllowedEmailNotFound, got %v", err)
		}
	})
}

func TestAdminStoreListAndRevokeSessions(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user := createAdminStoreUser(t, ctx, tdb, "store-session@example.com", "store-session")
		sessionOne := createAdminStoreSession(t, ctx, tdb, user.ID)
		sessionTwo := createAdminStoreSession(t, ctx, tdb, user.ID)

		sessions, err := tdb.Store.ListSessionsByUserID(ctx, user.ID)
		if err != nil {
			t.Fatalf("ListSessionsByUserID failed: %v", err)
		}
		if len(sessions) != 2 {
			t.Fatalf("expected 2 sessions, got %d", len(sessions))
		}

		if err := tdb.Store.RevokeSessionByIDAndUserID(ctx, sessionOne.ID, user.ID, adminStoreNow()); err != nil {
			t.Fatalf("RevokeSessionByIDAndUserID failed: %v", err)
		}
		active, err := tdb.Store.CountActiveSessionsByUserID(ctx, user.ID, adminStoreNow())
		if err != nil {
			t.Fatalf("CountActiveSessionsByUserID failed: %v", err)
		}
		if active != 1 {
			t.Fatalf("expected 1 active session, got %d", active)
		}

		revoked, err := tdb.Store.RevokeAllActiveSessionsForUser(ctx, user.ID, adminStoreNow())
		if err != nil {
			t.Fatalf("RevokeAllActiveSessionsForUser failed: %v", err)
		}
		if revoked != 1 {
			t.Fatalf("expected to revoke 1 remaining session, got %d", revoked)
		}
		updated, err := tdb.Store.GetSessionByID(ctx, sessionTwo.ID)
		if err != nil {
			t.Fatalf("GetSessionByID failed: %v", err)
		}
		if updated.RevokedAt == nil {
			t.Fatal("expected second session to be revoked")
		}
	})
}

func TestAdminStoreAuditEvents(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		actor := createAdminStoreUser(t, ctx, tdb, "store-audit-actor@example.com", "store-audit-actor")
		target := createAdminStoreUser(t, ctx, tdb, "store-audit-target@example.com", "store-audit-target")
		targetEmail := target.Email
		ip := "127.0.0.1"

		created, err := tdb.Store.CreateAdminAuditEvent(ctx, domain.AdminAuditEvent{
			ActorUserID:  &actor.ID,
			Action:       "user.disabled",
			TargetUserID: &target.ID,
			TargetEmail:  &targetEmail,
			IP:           &ip,
		})
		if err != nil {
			t.Fatalf("CreateAdminAuditEvent failed: %v", err)
		}
		if created.ID == uuid.Nil {
			t.Fatal("expected generated audit event id")
		}

		events, err := tdb.Store.ListAdminAuditEvents(ctx, store.AdminAuditEventFilter{
			ActorUserID: &actor.ID,
			Action:      "user.disabled",
		})
		if err != nil {
			t.Fatalf("ListAdminAuditEvents failed: %v", err)
		}
		if len(events) != 1 || events[0].ID != created.ID {
			t.Fatalf("expected created audit event, got %+v", events)
		}

		deleted, err := tdb.Store.DeleteAdminAuditEventsBefore(ctx, time.Now().Add(time.Hour))
		if err != nil {
			t.Fatalf("DeleteAdminAuditEventsBefore failed: %v", err)
		}
		if deleted != 1 {
			t.Fatalf("expected to delete 1 audit event, got %d", deleted)
		}
	})
}

func TestAdminStoreRecentFailures(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		challenge, err := tdb.Store.CreateChallenge(ctx, domain.Challenge{
			Purpose:      domain.ChallengePurposeSignup,
			Email:        "failure@example.com",
			ExpiresAt:    adminStoreNow().Add(-time.Minute),
			AttemptCount: 3,
			MaxAttempts:  3,
			ResendCount:  0,
			MaxResends:   2,
		})
		if err != nil {
			t.Fatalf("CreateChallenge failed: %v", err)
		}
		job, err := tdb.Store.CreateEmailJob(ctx, domain.EmailJob{
			ChallengeID:   &challenge.ID,
			ToEmail:       "failure@example.com",
			Template:      domain.EmailTemplateSignupCode,
			Status:        domain.EmailJobStatusPending,
			NextAttemptAt: adminStoreNow(),
		})
		if err != nil {
			t.Fatalf("CreateEmailJob failed: %v", err)
		}
		if err := tdb.Store.MarkEmailJobFailed(ctx, job.ID, "smtp failed"); err != nil {
			t.Fatalf("MarkEmailJobFailed failed: %v", err)
		}

		jobs, err := tdb.Store.ListRecentFailedEmailJobs(ctx, 10, 0)
		if err != nil {
			t.Fatalf("ListRecentFailedEmailJobs failed: %v", err)
		}
		if len(jobs) != 1 || jobs[0].ID != job.ID {
			t.Fatalf("expected failed email job, got %+v", jobs)
		}

		challenges, err := tdb.Store.ListRecentRiskyChallenges(ctx, adminStoreNow(), 10, 0)
		if err != nil {
			t.Fatalf("ListRecentRiskyChallenges failed: %v", err)
		}
		if len(challenges) != 1 || challenges[0].ID != challenge.ID {
			t.Fatalf("expected risky challenge, got %+v", challenges)
		}
	})
}

func createAdminStoreUser(t *testing.T, ctx context.Context, tdb *testutil.TestDB, email, username string) domain.User {
	t.Helper()

	user, err := tdb.Store.CreateUser(ctx, domain.User{
		Email:    email,
		Username: username,
	})
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	return user
}

func createAdminStoreSession(t *testing.T, ctx context.Context, tdb *testutil.TestDB, userID uuid.UUID) domain.Session {
	t.Helper()

	session, err := tdb.Store.CreateSession(ctx, domain.Session{
		UserID:    userID,
		ExpiresAt: adminStoreNow().Add(time.Hour),
		UserAgent: "admin-store-test",
	})
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	return session
}

func adminStoreNow() time.Time {
	return time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
}
