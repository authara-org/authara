package challenge

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/email"
	"github.com/authara-org/authara/internal/testutil"
)

func TestRunningChallengeServicesObservePolicyChangesWithoutReconstruction(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
		reader := newTestChallengePolicyReader(config.ChallengePolicy{
			TTL: 30 * time.Minute, VerificationCodeTTL: 10 * time.Minute,
			MaxAttempts: 5, MaxResends: 3, MinimumResendInterval: 30 * time.Second,
		})
		service := New(Config{Store: tdb.Store, Tx: tdb.Tx, Policy: reader})
		verification := NewVerificationCodeServiceWithPolicy(
			tdb.Store,
			reader,
			[]byte("01234567890123456789012345678901"),
		)

		firstID, err := service.createChallenge(
			ctx, domain.ChallengePurposeSignup, "first-policy@example.com", now,
			func(context.Context, domain.Challenge) error { return nil },
		)
		if err != nil {
			t.Fatal(err)
		}
		first, err := tdb.Store.GetChallengeByID(ctx, firstID)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := verification.GenerateCode(ctx, first, now); err != nil {
			t.Fatal(err)
		}
		firstCode, err := tdb.Store.GetVerificationCodeByChallengeID(ctx, firstID)
		if err != nil {
			t.Fatal(err)
		}

		reader.Store(config.ChallengePolicy{
			TTL: 2 * time.Hour, VerificationCodeTTL: 20 * time.Minute,
			MaxAttempts: 9, MaxResends: 4, MinimumResendInterval: 2 * time.Minute,
		})
		secondID, err := service.createChallenge(
			ctx, domain.ChallengePurposeSignup, "second-policy@example.com", now,
			func(context.Context, domain.Challenge) error { return nil },
		)
		if err != nil {
			t.Fatal(err)
		}
		second, err := tdb.Store.GetChallengeByID(ctx, secondID)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := verification.GenerateCode(ctx, second, now); err != nil {
			t.Fatal(err)
		}
		secondCode, err := tdb.Store.GetVerificationCodeByChallengeID(ctx, secondID)
		if err != nil {
			t.Fatal(err)
		}

		if !first.ExpiresAt.Equal(now.Add(30*time.Minute)) || first.MaxAttempts != 5 || first.MaxResends != 3 || first.MinimumResendInterval != 30*time.Second || !first.HasMinimumResendInterval {
			t.Fatalf("first artifact changed or was created with the wrong policy: %+v", first)
		}
		if !firstCode.ExpiresAt.Equal(now.Add(10 * time.Minute)) {
			t.Fatalf("first code expiry = %s", firstCode.ExpiresAt)
		}
		if !second.ExpiresAt.Equal(now.Add(2*time.Hour)) || second.MaxAttempts != 9 || second.MaxResends != 4 || second.MinimumResendInterval != 2*time.Minute || !second.HasMinimumResendInterval {
			t.Fatalf("second artifact did not use updated policy: %+v", second)
		}
		if !secondCode.ExpiresAt.Equal(now.Add(20 * time.Minute)) {
			t.Fatalf("second code expiry = %s", secondCode.ExpiresAt)
		}
	})
}

func TestLegacyChallengeUsesCurrentResendIntervalFallback(t *testing.T) {
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	lastSentAt := now.Add(-30 * time.Second)
	service := &Service{}
	legacy := domain.Challenge{ExpiresAt: now.Add(time.Hour), MaxResends: 1, LastSentAt: &lastSentAt}
	if err := service.validateChallengeForResend(legacy, now, time.Minute); !errors.Is(err, ErrResendTooSoon) {
		t.Fatalf("legacy resend validation = %v", err)
	}

	stored := legacy
	stored.HasMinimumResendInterval = true
	stored.MinimumResendInterval = 10 * time.Second
	if err := service.validateChallengeForResend(stored, now, time.Minute); err != nil {
		t.Fatalf("stored resend validation = %v", err)
	}
}

type testChallengePolicyReader struct {
	current atomic.Pointer[config.ChallengePolicy]
}

func newTestChallengePolicyReader(policy config.ChallengePolicy) *testChallengePolicyReader {
	reader := &testChallengePolicyReader{}
	reader.Store(policy)
	return reader
}

func (r *testChallengePolicyReader) CurrentChallenge() config.ChallengePolicy {
	return *r.current.Load()
}

func (r *testChallengePolicyReader) Store(policy config.ChallengePolicy) {
	copy := policy
	r.current.Store(&copy)
}

func TestOpaqueChallengeCannotBeResent(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
		svc := New(Config{
			Store:             tdb.Store,
			Tx:                tdb.Tx,
			ChallengeTTL:      30 * time.Minute,
			MaxAttempts:       5,
			MaxResends:        3,
			MinResendInterval: time.Second,
		})

		challengeID, err := svc.CreateOpaqueChallenge(ctx, now, domain.ChallengePurposeSignup, "opaque@example.com")
		if err != nil {
			t.Fatalf("CreateOpaqueChallenge failed: %v", err)
		}

		row, err := tdb.Store.GetChallengeByID(ctx, challengeID)
		if err != nil {
			t.Fatalf("GetChallengeByID failed: %v", err)
		}
		if row.MaxResends != 0 {
			t.Fatalf("expected opaque challenge max_resends=0, got %d", row.MaxResends)
		}

		err = svc.ResendChallenge(ctx, challengeID, now.Add(time.Minute))
		if !errors.Is(err, ErrTooManyResends) {
			t.Fatalf("expected ErrTooManyResends, got %v", err)
		}
	})
}

func TestSignupChallengeStoresInvitationID(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
		owner, err := tdb.Store.CreateUser(ctx, domain.User{Email: "invite-owner@example.com", Username: "invite-owner"})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}
		org, _, err := tdb.Store.EnsureOrganizationForUser(ctx, owner.ID, owner.Username, domain.OrganizationKindTeam)
		if err != nil {
			t.Fatalf("EnsureOrganizationForUser failed: %v", err)
		}
		invitation, err := tdb.Store.CreateOrganizationInvitation(ctx, domain.OrganizationInvitation{
			OrganizationID: org.ID,
			Email:          "invitee@example.com",
			Role:           domain.OrganizationRoleMember,
			TokenHash:      "pending-signup-test-token-hash",
			ExpiresAt:      now.Add(time.Hour),
		})
		if err != nil {
			t.Fatalf("CreateOrganizationInvitation failed: %v", err)
		}
		svc := New(Config{
			Store:             tdb.Store,
			Tx:                tdb.Tx,
			ChallengeTTL:      30 * time.Minute,
			MaxAttempts:       5,
			MaxResends:        3,
			MinResendInterval: time.Second,
		})

		challengeID, err := svc.CreateSignupChallenge(ctx, CreateSignupChallengeInput{
			Email:        "invitee@example.com",
			PasswordHash: "hash",
			InvitationID: &invitation.ID,
		}, now)
		if err != nil {
			t.Fatalf("CreateSignupChallenge failed: %v", err)
		}

		action, err := tdb.Store.GetPendingSignupActionByChallengeID(ctx, challengeID)
		if err != nil {
			t.Fatalf("GetPendingSignupActionByChallengeID failed: %v", err)
		}
		if action.InvitationID == nil || *action.InvitationID != invitation.ID {
			t.Fatalf("expected invitation id %q to round trip, got %v", invitation.ID, action.InvitationID)
		}
	})
}

func TestVerifyChallengeWrongPurposeDoesNotConsumeIt(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
		svc := New(Config{
			Store:        tdb.Store,
			Tx:           tdb.Tx,
			ChallengeTTL: 30 * time.Minute,
			MaxAttempts:  5,
			MaxResends:   3,
		})
		verifier := NewVerificationCodeService(
			tdb.Store,
			10*time.Minute,
			[]byte("01234567890123456789012345678901"),
		)

		challengeID, err := svc.CreateSignupChallenge(ctx, CreateSignupChallengeInput{
			Email:        "wrong-purpose@example.com",
			PasswordHash: "hash",
		}, now)
		if err != nil {
			t.Fatalf("CreateSignupChallenge failed: %v", err)
		}
		row, err := tdb.Store.GetChallengeByID(ctx, challengeID)
		if err != nil {
			t.Fatalf("GetChallengeByID failed: %v", err)
		}
		code, err := verifier.GenerateCode(ctx, row, now)
		if err != nil {
			t.Fatalf("GenerateCode failed: %v", err)
		}

		_, err = svc.VerifyPasswordResetChallenge(ctx, challengeID, code, verifier, now)
		if !errors.Is(err, ErrUnsupportedChallengePurpose) {
			t.Fatalf("expected ErrUnsupportedChallengePurpose, got %v", err)
		}

		row, err = tdb.Store.GetChallengeByID(ctx, challengeID)
		if err != nil {
			t.Fatalf("GetChallengeByID failed: %v", err)
		}
		if row.ConsumedAt != nil || row.AttemptCount != 0 {
			t.Fatalf("wrong purpose changed challenge: consumed_at=%v attempt_count=%d", row.ConsumedAt, row.AttemptCount)
		}
	})
}

func TestSignupCompletionFailureDoesNotConsumeChallenge(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
		svc := New(Config{
			Store:        tdb.Store,
			Tx:           tdb.Tx,
			ChallengeTTL: 30 * time.Minute,
			MaxAttempts:  5,
			MaxResends:   3,
		})
		verifier := NewVerificationCodeService(
			tdb.Store,
			10*time.Minute,
			[]byte("01234567890123456789012345678901"),
		)

		challengeID, err := svc.CreateSignupChallenge(ctx, CreateSignupChallengeInput{
			Email:        "retry-signup@example.com",
			PasswordHash: "hash",
		}, now)
		if err != nil {
			t.Fatalf("CreateSignupChallenge failed: %v", err)
		}
		row, err := tdb.Store.GetChallengeByID(ctx, challengeID)
		if err != nil {
			t.Fatalf("GetChallengeByID failed: %v", err)
		}
		code, err := verifier.GenerateCode(ctx, row, now)
		if err != nil {
			t.Fatalf("GenerateCode failed: %v", err)
		}

		completionErr := errors.New("signup completion failed")
		_, err = svc.VerifySignupChallenge(
			ctx,
			challengeID,
			code,
			verifier,
			now,
			func(context.Context, domain.PendingSignupAction) error { return completionErr },
		)
		if !errors.Is(err, completionErr) {
			t.Fatalf("expected completion error, got %v", err)
		}

		row, err = tdb.Store.GetChallengeByID(ctx, challengeID)
		if err != nil {
			t.Fatalf("GetChallengeByID failed: %v", err)
		}
		if row.ConsumedAt != nil || row.AttemptCount != 0 {
			t.Fatalf("completion failure changed challenge: consumed_at=%v attempt_count=%d", row.ConsumedAt, row.AttemptCount)
		}

		if _, err := svc.VerifySignupChallenge(ctx, challengeID, code, verifier, now, nil); err != nil {
			t.Fatalf("valid retry failed: %v", err)
		}
	})
}

func TestVerificationServiceFailureDoesNotIncrementAttempts(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
		svc := New(Config{
			Store:        tdb.Store,
			Tx:           tdb.Tx,
			ChallengeTTL: 30 * time.Minute,
			MaxAttempts:  5,
			MaxResends:   3,
		})
		verifier := NewVerificationCodeService(
			tdb.Store,
			10*time.Minute,
			[]byte("01234567890123456789012345678901"),
		)

		challengeID, err := svc.CreateSignupChallenge(ctx, CreateSignupChallengeInput{
			Email:        "verification-service-error@example.com",
			PasswordHash: "hash",
		}, now)
		if err != nil {
			t.Fatalf("CreateSignupChallenge failed: %v", err)
		}
		row, err := tdb.Store.GetChallengeByID(ctx, challengeID)
		if err != nil {
			t.Fatalf("GetChallengeByID failed: %v", err)
		}
		code, err := verifier.GenerateCode(ctx, row, now)
		if err != nil {
			t.Fatalf("GenerateCode failed: %v", err)
		}

		unconfiguredVerifier := NewVerificationCodeService(tdb.Store, 10*time.Minute)
		_, err = svc.VerifySignupChallenge(ctx, challengeID, code, unconfiguredVerifier, now, nil)
		if err == nil {
			t.Fatal("expected verification service error")
		}

		row, err = tdb.Store.GetChallengeByID(ctx, challengeID)
		if err != nil {
			t.Fatalf("GetChallengeByID failed: %v", err)
		}
		if row.ConsumedAt != nil || row.AttemptCount != 0 {
			t.Fatalf("service failure changed challenge: consumed_at=%v attempt_count=%d", row.ConsumedAt, row.AttemptCount)
		}
	})
}

func TestExecuteEmailChangeMovesAllowlistEntryWhenEnabled(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
		oldEmail := "email-change-old@example.com"
		newEmail := "email-change-new@example.com"
		user, err := tdb.Store.CreateUser(ctx, domain.User{Email: oldEmail, Username: "email-change-allowlist"})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}
		if err := tdb.Store.EnsureAllowedEmail(ctx, oldEmail); err != nil {
			t.Fatalf("EnsureAllowedEmail failed: %v", err)
		}

		svc := New(Config{
			Store:            tdb.Store,
			Tx:               tdb.Tx,
			AllowlistEnabled: true,
			ChallengeTTL:     30 * time.Minute,
			MaxAttempts:      5,
			MaxResends:       3,
		})
		challengeID, err := svc.CreateEmailChangeChallenge(ctx, CreateEmailChangeChallengeInput{
			UserID:   user.ID,
			OldEmail: oldEmail,
			NewEmail: newEmail,
		}, now)
		if err != nil {
			t.Fatalf("CreateEmailChangeChallenge failed: %v", err)
		}
		action, err := tdb.Store.GetPendingEmailChangeByChallengeID(ctx, challengeID)
		if err != nil {
			t.Fatalf("GetPendingEmailChangeByChallengeID failed: %v", err)
		}

		if err := svc.ExecuteEmailChange(ctx, action, now); err != nil {
			t.Fatalf("ExecuteEmailChange failed: %v", err)
		}

		updatedUser, err := tdb.Store.GetUserByID(ctx, user.ID)
		if err != nil {
			t.Fatalf("GetUserByID failed: %v", err)
		}
		if updatedUser.Email != newEmail {
			t.Fatalf("expected user email %q, got %q", newEmail, updatedUser.Email)
		}
		oldAllowed, err := tdb.Store.IsEmailAllowed(ctx, oldEmail)
		if err != nil {
			t.Fatalf("IsEmailAllowed(old) failed: %v", err)
		}
		newAllowed, err := tdb.Store.IsEmailAllowed(ctx, newEmail)
		if err != nil {
			t.Fatalf("IsEmailAllowed(new) failed: %v", err)
		}
		if oldAllowed || !newAllowed {
			t.Fatalf("expected allowlist to move from old to new email, old=%t new=%t", oldAllowed, newAllowed)
		}
		if got := testutil.CountEmailJobs(t, ctx, oldEmail, domain.EmailTemplateEmailChangedOldAddress); got != 1 {
			t.Fatalf("old-address confirmation email jobs = %d, want 1", got)
		}
		if got := testutil.CountEmailJobs(t, ctx, newEmail, domain.EmailTemplateEmailChangedNewAddress); got != 1 {
			t.Fatalf("new-address confirmation email jobs = %d, want 1", got)
		}
		data := testutil.LatestEmailTemplateData(t, ctx, oldEmail, domain.EmailTemplateEmailChangedOldAddress)
		if data[email.TemplateVariableOldEmail] != oldEmail ||
			data[email.TemplateVariableNewEmail] != newEmail ||
			data[email.TemplateVariableOccurredAt] != "2026-07-17T12:00:00Z" {
			t.Fatalf("unexpected email-change template data: %#v", data)
		}
	})
}

func TestExecutePasswordResetQueuesPasswordChangedNotification(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Date(2026, 9, 9, 18, 30, 0, 0, time.UTC)
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "password-reset-notification@example.com",
			Username: "password-reset-notification",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}
		oldHash := "old-password-hash"
		if _, err := tdb.Store.CreateAuthProvider(ctx, domain.AuthProvider{
			UserID:       user.ID,
			Provider:     domain.ProviderPassword,
			PasswordHash: &oldHash,
		}); err != nil {
			t.Fatalf("CreateAuthProvider failed: %v", err)
		}
		svc := New(Config{
			Store:        tdb.Store,
			Tx:           tdb.Tx,
			ChallengeTTL: 30 * time.Minute,
			MaxAttempts:  5,
			MaxResends:   3,
		})
		challengeID, err := svc.CreatePasswordResetChallenge(ctx, CreatePasswordResetChallengeInput{
			UserID:       user.ID,
			Email:        user.Email,
			PasswordHash: "new-password-hash",
		}, now)
		if err != nil {
			t.Fatalf("CreatePasswordResetChallenge failed: %v", err)
		}
		action, err := tdb.Store.GetPendingPasswordResetByChallengeID(ctx, challengeID)
		if err != nil {
			t.Fatalf("GetPendingPasswordResetByChallengeID failed: %v", err)
		}
		if err := svc.ExecutePasswordReset(ctx, action, now); err != nil {
			t.Fatalf("ExecutePasswordReset failed: %v", err)
		}
		if got := testutil.CountEmailJobs(t, ctx, user.Email, domain.EmailTemplatePasswordChanged); got != 1 {
			t.Fatalf("password-changed email jobs = %d, want 1", got)
		}
	})
}
