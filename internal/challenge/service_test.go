package challenge

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/cache"
	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/email"
	"github.com/authara-org/authara/internal/session/token"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/authara-org/authara/internal/webhook"
	"github.com/google/uuid"
)

type challengePublisherFunc func(context.Context, webhook.Envelope) error

func (f challengePublisherFunc) Publish(ctx context.Context, event webhook.Envelope) error {
	return f(ctx, event)
}

type toggleFailureCache struct {
	cache.Noop
	fail atomic.Bool
	err  error
}

func (c *toggleFailureCache) Set(context.Context, string, []byte, time.Duration) error {
	if c.fail.Load() {
		return c.err
	}
	return nil
}

func (c *toggleFailureCache) SetMaxInt64(context.Context, string, int64, time.Duration) error {
	if c.fail.Load() {
		return c.err
	}
	return nil
}

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

		err = svc.CompletePasswordResetChallenge(ctx, challengeID, code, verifier, now)
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
		session := createEmailChangeTestSession(t, ctx, tdb, user, now)

		svc := New(Config{
			Store:            tdb.Store,
			Tx:               tdb.Tx,
			AllowlistEnabled: true,
			ChallengeTTL:     30 * time.Minute,
			MaxAttempts:      5,
			MaxResends:       3,
		})
		challengeID, err := svc.CreateEmailChangeChallenge(ctx, CreateEmailChangeChallengeInput{
			UserID:              user.ID,
			InitiatingSessionID: session.ID,
			OldEmail:            oldEmail,
			NewEmail:            newEmail,
		}, now)
		if err != nil {
			t.Fatalf("CreateEmailChangeChallenge failed: %v", err)
		}
		action, err := tdb.Store.GetPendingEmailChangeByChallengeID(ctx, challengeID)
		if err != nil {
			t.Fatalf("GetPendingEmailChangeByChallengeID failed: %v", err)
		}

		if err := svc.executeEmailChange(ctx, action, now); err != nil {
			t.Fatalf("executeEmailChange failed: %v", err)
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

func TestCompleteEmailChangeRejectsAnotherSessionBeforeConsumingChallenge(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
		user, err := tdb.Store.CreateUser(ctx, domain.User{Email: "session-bound-old@example.com", Username: "session-bound"})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}
		initiatingSession := createEmailChangeTestSession(t, ctx, tdb, user, now)
		otherSession, err := tdb.Store.CreateSession(ctx, domain.Session{
			UserID:               user.ID,
			ActiveOrganizationID: initiatingSession.ActiveOrganizationID,
			ExpiresAt:            now.Add(time.Hour),
			UserAgent:            "other",
		})
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}

		svc := New(Config{Store: tdb.Store, Tx: tdb.Tx, ChallengeTTL: 30 * time.Minute, MaxAttempts: 5, MaxResends: 3})
		verification := NewVerificationCodeService(tdb.Store, 10*time.Minute, []byte("01234567890123456789012345678901"))
		challengeID, code := createEmailChangeTestChallenge(t, ctx, tdb, svc, verification, user, initiatingSession.ID, "session-bound-new@example.com", now)

		err = svc.CompleteEmailChangeChallenge(ctx, CompleteEmailChangeChallengeInput{
			ChallengeID: challengeID,
			UserID:      user.ID,
			SessionID:   otherSession.ID,
			Code:        code,
		}, verification, now)
		if !errors.Is(err, ErrEmailChangeNotAuthorized) {
			t.Fatalf("cross-session completion error = %v", err)
		}
		row, err := tdb.Store.GetChallengeByID(ctx, challengeID)
		if err != nil {
			t.Fatalf("GetChallengeByID failed: %v", err)
		}
		if row.ConsumedAt != nil || row.AttemptCount != 0 {
			t.Fatalf("cross-session attempt changed challenge: consumed_at=%v attempt_count=%d", row.ConsumedAt, row.AttemptCount)
		}

		wrongCode := "000000"
		if code == wrongCode {
			wrongCode = "111111"
		}
		err = svc.CompleteEmailChangeChallenge(ctx, CompleteEmailChangeChallengeInput{
			ChallengeID: challengeID,
			UserID:      user.ID,
			SessionID:   initiatingSession.ID,
			Code:        wrongCode,
		}, verification, now)
		if !errors.Is(err, ErrInvalidVerificationCode) {
			t.Fatalf("wrong-code completion error = %v", err)
		}
		row, err = tdb.Store.GetChallengeByID(ctx, challengeID)
		if err != nil {
			t.Fatalf("GetChallengeByID failed: %v", err)
		}
		if row.AttemptCount != 1 || row.ConsumedAt != nil {
			t.Fatalf("wrong-code challenge state: attempts=%d consumed_at=%v", row.AttemptCount, row.ConsumedAt)
		}

		if err := svc.CompleteEmailChangeChallenge(ctx, CompleteEmailChangeChallengeInput{
			ChallengeID: challengeID,
			UserID:      user.ID,
			SessionID:   initiatingSession.ID,
			Code:        code,
		}, verification, now); err != nil {
			t.Fatalf("initiating-session completion failed: %v", err)
		}
	})
}

func TestCompleteEmailChangeRejectsRevokedSession(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Date(2026, 9, 13, 13, 0, 0, 0, time.UTC)
		user, err := tdb.Store.CreateUser(ctx, domain.User{Email: "revoked-old@example.com", Username: "revoked-email-change"})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}
		session := createEmailChangeTestSession(t, ctx, tdb, user, now)
		svc := New(Config{Store: tdb.Store, Tx: tdb.Tx, ChallengeTTL: 30 * time.Minute, MaxAttempts: 5, MaxResends: 3})
		verification := NewVerificationCodeService(tdb.Store, 10*time.Minute, []byte("01234567890123456789012345678901"))
		challengeID, code := createEmailChangeTestChallenge(t, ctx, tdb, svc, verification, user, session.ID, "revoked-new@example.com", now)

		if err := tdb.Store.RevokeSession(ctx, session.ID, now.Add(time.Minute)); err != nil {
			t.Fatalf("RevokeSession failed: %v", err)
		}
		err = svc.CompleteEmailChangeChallenge(ctx, CompleteEmailChangeChallengeInput{
			ChallengeID: challengeID,
			UserID:      user.ID,
			SessionID:   session.ID,
			Code:        code,
		}, verification, now.Add(2*time.Minute))
		if !errors.Is(err, ErrEmailChangeNotAuthorized) {
			t.Fatalf("revoked-session completion error = %v", err)
		}
		updated, err := tdb.Store.GetUserByID(ctx, user.ID)
		if err != nil {
			t.Fatalf("GetUserByID failed: %v", err)
		}
		if updated.Email != user.Email {
			t.Fatalf("email changed after revocation: %q", updated.Email)
		}
		if _, err := tdb.Store.GetPendingEmailChangeByChallengeID(ctx, challengeID); !errors.Is(err, store.ErrorPendingEmailChangeNotFound) {
			t.Fatalf("pending email change survived revocation: %v", err)
		}
		if _, err := svc.CreateEmailChangeChallenge(ctx, CreateEmailChangeChallengeInput{
			UserID:              user.ID,
			InitiatingSessionID: session.ID,
			OldEmail:            user.Email,
			NewEmail:            "revoked-new@example.com",
		}, now.Add(2*time.Minute)); !errors.Is(err, ErrEmailChangeNotAuthorized) {
			t.Fatalf("email change started from revoked session: %v", err)
		}
	})
}

func TestCompleteEmailChangeRejectsDisabledUser(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Date(2026, 9, 13, 13, 30, 0, 0, time.UTC)
		user, err := tdb.Store.CreateUser(ctx, domain.User{Email: "disabled-old@example.com", Username: "disabled-email-change"})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}
		session := createEmailChangeTestSession(t, ctx, tdb, user, now)
		svc := New(Config{Store: tdb.Store, Tx: tdb.Tx, ChallengeTTL: 30 * time.Minute, MaxAttempts: 5, MaxResends: 3})
		verification := NewVerificationCodeService(tdb.Store, 10*time.Minute, []byte("01234567890123456789012345678901"))
		challengeID, code := createEmailChangeTestChallenge(t, ctx, tdb, svc, verification, user, session.ID, "disabled-new@example.com", now)

		if err := tdb.Store.DisableUser(ctx, user.ID, now.Add(time.Minute)); err != nil {
			t.Fatalf("DisableUser failed: %v", err)
		}
		err = svc.CompleteEmailChangeChallenge(ctx, CompleteEmailChangeChallengeInput{
			ChallengeID: challengeID,
			UserID:      user.ID,
			SessionID:   session.ID,
			Code:        code,
		}, verification, now.Add(2*time.Minute))
		if !errors.Is(err, ErrEmailChangeNotAuthorized) {
			t.Fatalf("disabled-user completion error = %v", err)
		}
		if _, err := tdb.Store.GetPendingEmailChangeByChallengeID(ctx, challengeID); !errors.Is(err, store.ErrorPendingEmailChangeNotFound) {
			t.Fatalf("pending email change survived account disable: %v", err)
		}
	})
}

func createEmailChangeTestSession(t *testing.T, ctx context.Context, tdb *testutil.TestDB, user domain.User, now time.Time) domain.Session {
	t.Helper()

	org, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username)
	if err != nil {
		t.Fatalf("EnsureDefaultOrganizationForUser failed: %v", err)
	}
	session, err := tdb.Store.CreateSession(ctx, domain.Session{
		UserID:               user.ID,
		ActiveOrganizationID: org.ID,
		ExpiresAt:            now.Add(time.Hour),
		UserAgent:            "initiating",
	})
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	return session
}

func createEmailChangeTestChallenge(
	t *testing.T,
	ctx context.Context,
	tdb *testutil.TestDB,
	svc *Service,
	verification *VerificationCodeService,
	user domain.User,
	sessionID uuid.UUID,
	newEmail string,
	now time.Time,
) (uuid.UUID, string) {
	t.Helper()

	challengeID, err := svc.CreateEmailChangeChallenge(ctx, CreateEmailChangeChallengeInput{
		UserID:              user.ID,
		InitiatingSessionID: sessionID,
		OldEmail:            user.Email,
		NewEmail:            newEmail,
	}, now)
	if err != nil {
		t.Fatalf("CreateEmailChangeChallenge failed: %v", err)
	}
	row, err := tdb.Store.GetChallengeByID(ctx, challengeID)
	if err != nil {
		t.Fatalf("GetChallengeByID failed: %v", err)
	}
	code, err := verification.GenerateCode(ctx, row, now)
	if err != nil {
		t.Fatalf("GenerateCode failed: %v", err)
	}
	return challengeID, code
}

func TestCreatePasswordResetChallengeUsesOpaqueChallengeWithoutPasswordProvider(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Date(2026, 9, 9, 18, 0, 0, 0, time.UTC)
		svc := New(Config{
			Store:        tdb.Store,
			Tx:           tdb.Tx,
			ChallengeTTL: 30 * time.Minute,
			MaxAttempts:  5,
			MaxResends:   3,
		})
		verification := NewVerificationCodeService(tdb.Store, 10*time.Minute, []byte("01234567890123456789012345678901"))
		for _, method := range []string{"oauth", "passkey"} {
			t.Run(method, func(t *testing.T) {
				user, err := tdb.Store.CreateUser(ctx, domain.User{
					Email:    "password-reset-" + method + "@example.com",
					Username: "password-reset-" + method,
				})
				if err != nil {
					t.Fatalf("CreateUser failed: %v", err)
				}
				if method == "oauth" {
					providerUserID := "google-password-reset-user"
					_, err = tdb.Store.CreateAuthProvider(ctx, domain.AuthProvider{
						UserID: user.ID, Provider: domain.ProviderGoogle, ProviderUserID: &providerUserID,
					})
				} else {
					_, err = tdb.Store.CreatePasskey(ctx, domain.Passkey{
						UserID: user.ID, CredentialID: []byte("password-reset-passkey"), PublicKey: []byte("public-key"),
					})
				}
				if err != nil {
					t.Fatalf("create %s method: %v", method, err)
				}

				challengeID, err := svc.CreatePasswordResetChallenge(ctx, CreatePasswordResetChallengeInput{
					UserID:       user.ID,
					Email:        user.Email,
					PasswordHash: "new-password-hash",
				}, now)
				if err != nil {
					t.Fatalf("CreatePasswordResetChallenge failed: %v", err)
				}

				row, err := tdb.Store.GetChallengeByID(ctx, challengeID)
				if err != nil {
					t.Fatalf("GetChallengeByID failed: %v", err)
				}
				if row.MaxResends != 0 {
					t.Fatalf("passwordless challenge max_resends = %d, want 0", row.MaxResends)
				}
				if _, err := tdb.Store.GetPendingPasswordResetByChallengeID(ctx, challengeID); !errors.Is(err, store.ErrorPendingPasswordResetNotFound) {
					t.Fatalf("passwordless challenge created pending reset: %v", err)
				}
				if got := testutil.CountEmailJobs(t, ctx, user.Email, domain.EmailTemplatePasswordResetCode); got != 0 {
					t.Fatalf("passwordless reset-code email jobs = %d, want 0", got)
				}

				// Even an injected verification code cannot turn an opaque
				// challenge into authority to create a password provider.
				code, err := verification.GenerateCode(ctx, row, now)
				if err != nil {
					t.Fatalf("GenerateCode failed: %v", err)
				}
				if err := svc.CompletePasswordResetChallenge(ctx, challengeID, code, verification, now); !errors.Is(err, ErrPasswordResetUnavailable) {
					t.Fatalf("opaque completion error = %v, want ErrPasswordResetUnavailable", err)
				}
				if _, err := tdb.Store.GetAuthProviderByMethodAndUserID(ctx, domain.ProviderPassword, user.ID); !errors.Is(err, store.ErrorAuthProviderNotFound) {
					t.Fatalf("opaque completion linked a password provider: %v", err)
				}
			})
		}
	})
}

func TestPasswordResetCompletionFailureDoesNotConsumeChallenge(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Date(2026, 9, 9, 18, 15, 0, 0, time.UTC)
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "password-reset-retry@example.com",
			Username: "password-reset-retry",
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
		verification := NewVerificationCodeService(
			tdb.Store,
			10*time.Minute,
			[]byte("01234567890123456789012345678901"),
		)
		challengeID, err := svc.CreatePasswordResetChallenge(ctx, CreatePasswordResetChallengeInput{
			UserID:       user.ID,
			Email:        user.Email,
			PasswordHash: "new-password-hash",
		}, now)
		if err != nil {
			t.Fatalf("CreatePasswordResetChallenge failed: %v", err)
		}
		row, err := tdb.Store.GetChallengeByID(ctx, challengeID)
		if err != nil {
			t.Fatalf("GetChallengeByID failed: %v", err)
		}
		code, err := verification.GenerateCode(ctx, row, now)
		if err != nil {
			t.Fatalf("GenerateCode failed: %v", err)
		}

		if err := tdb.Store.DeleteAuthProviderByMethodAndUserID(ctx, domain.ProviderPassword, user.ID); err != nil {
			t.Fatalf("DeleteAuthProviderByMethodAndUserID failed: %v", err)
		}
		err = svc.CompletePasswordResetChallenge(ctx, challengeID, code, verification, now)
		if !errors.Is(err, ErrPasswordResetUnavailable) {
			t.Fatalf("expected ErrPasswordResetUnavailable, got %v", err)
		}

		row, err = tdb.Store.GetChallengeByID(ctx, challengeID)
		if err != nil {
			t.Fatalf("GetChallengeByID failed: %v", err)
		}
		if row.ConsumedAt != nil || row.AttemptCount != 0 {
			t.Fatalf("failed completion changed challenge: consumed_at=%v attempt_count=%d", row.ConsumedAt, row.AttemptCount)
		}
		if _, err := tdb.Store.GetPendingPasswordResetByChallengeID(ctx, challengeID); err != nil {
			t.Fatalf("failed completion removed pending reset: %v", err)
		}
		if _, err := tdb.Store.GetAuthProviderByMethodAndUserID(ctx, domain.ProviderPassword, user.ID); !errors.Is(err, store.ErrorAuthProviderNotFound) {
			t.Fatalf("failed completion linked a password provider: %v", err)
		}
		if got := testutil.CountEmailJobs(t, ctx, user.Email, domain.EmailTemplatePasswordChanged); got != 0 {
			t.Fatalf("failed completion queued password-changed jobs = %d, want 0", got)
		}
	})
}

func TestCompletePasswordResetChallengeQueuesPasswordChangedNotification(t *testing.T) {
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
		verification := NewVerificationCodeService(
			tdb.Store,
			10*time.Minute,
			[]byte("01234567890123456789012345678901"),
		)
		challengeID, err := svc.CreatePasswordResetChallenge(ctx, CreatePasswordResetChallengeInput{
			UserID:       user.ID,
			Email:        user.Email,
			PasswordHash: "new-password-hash",
		}, now)
		if err != nil {
			t.Fatalf("CreatePasswordResetChallenge failed: %v", err)
		}
		row, err := tdb.Store.GetChallengeByID(ctx, challengeID)
		if err != nil {
			t.Fatalf("GetChallengeByID failed: %v", err)
		}
		code, err := verification.GenerateCode(ctx, row, now)
		if err != nil {
			t.Fatalf("GenerateCode failed: %v", err)
		}
		if err := svc.CompletePasswordResetChallenge(ctx, challengeID, code, verification, now); err != nil {
			t.Fatalf("CompletePasswordResetChallenge failed: %v", err)
		}
		if got := testutil.CountEmailJobs(t, ctx, user.Email, domain.EmailTemplatePasswordChanged); got != 1 {
			t.Fatalf("password-changed email jobs = %d, want 1", got)
		}
	})
}

func TestNewPasswordResetSupersedesPreviousReset(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
		user := createPendingActionTestUser(t, ctx, tdb, "superseded-reset@example.com")
		svc, verifier := newPendingActionTestServices(tdb)

		firstID, firstCode := createPasswordResetTestChallenge(t, ctx, tdb, svc, verifier, user, "first-hash", now)
		secondID, secondCode := createPasswordResetTestChallenge(t, ctx, tdb, svc, verifier, user, "second-hash", now.Add(time.Minute))

		if _, err := tdb.Store.GetPendingPasswordResetByChallengeID(ctx, firstID); !errors.Is(err, store.ErrorPendingPasswordResetNotFound) {
			t.Fatalf("first reset remained authoritative: %v", err)
		}
		if err := svc.ResendChallenge(ctx, firstID, now.Add(2*time.Minute)); !errors.Is(err, ErrChallengeConsumed) {
			t.Fatalf("superseded reset resend error = %v", err)
		}
		if got := testutil.CountEmailJobs(t, ctx, user.Email, domain.EmailTemplatePasswordResetCode); got != 2 {
			t.Fatalf("reset code email jobs after stale resend = %d, want 2", got)
		}
		if err := svc.CompletePasswordResetChallenge(ctx, firstID, firstCode, verifier, now.Add(2*time.Minute)); !errors.Is(err, ErrPasswordResetUnavailable) {
			t.Fatalf("superseded reset completion error = %v", err)
		}
		assertPasswordHash(t, ctx, tdb, user.ID, "old-hash")

		if err := svc.CompletePasswordResetChallenge(ctx, secondID, secondCode, verifier, now.Add(2*time.Minute)); err != nil {
			t.Fatalf("latest reset failed: %v", err)
		}
		assertPasswordHash(t, ctx, tdb, user.ID, "second-hash")
	})
}

func TestNewEmailChangeSupersedesPreviousAndInvalidatesReset(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Date(2026, 9, 15, 11, 0, 0, 0, time.UTC)
		user := createPendingActionTestUser(t, ctx, tdb, "superseded-email@example.com")
		session := createEmailChangeTestSession(t, ctx, tdb, user, now)
		svc, verifier := newPendingActionTestServices(tdb)

		firstID, firstCode := createEmailChangeTestChallenge(t, ctx, tdb, svc, verifier, user, session.ID, "first-email@example.com", now)
		secondID, secondCode := createEmailChangeTestChallenge(t, ctx, tdb, svc, verifier, user, session.ID, "second-email@example.com", now.Add(time.Minute))
		resetID, resetCode := createPasswordResetTestChallenge(t, ctx, tdb, svc, verifier, user, "reset-hash", now.Add(time.Minute))

		if _, err := tdb.Store.GetPendingEmailChangeByChallengeID(ctx, firstID); !errors.Is(err, store.ErrorPendingEmailChangeNotFound) {
			t.Fatalf("first email change remained authoritative: %v", err)
		}
		if err := svc.ResendChallenge(ctx, firstID, now.Add(2*time.Minute)); !errors.Is(err, ErrChallengeConsumed) {
			t.Fatalf("superseded email change resend error = %v", err)
		}
		if got := testutil.CountEmailJobs(t, ctx, "first-email@example.com", domain.EmailTemplateEmailChangeCode); got != 1 {
			t.Fatalf("email-change code jobs after stale resend = %d, want 1", got)
		}
		if err := svc.CompleteEmailChangeChallenge(ctx, CompleteEmailChangeChallengeInput{
			ChallengeID: firstID, UserID: user.ID, SessionID: session.ID, Code: firstCode,
		}, verifier, now.Add(2*time.Minute)); !errors.Is(err, ErrEmailChangeNotAuthorized) {
			t.Fatalf("superseded email change completion error = %v", err)
		}

		if err := svc.CompleteEmailChangeChallenge(ctx, CompleteEmailChangeChallengeInput{
			ChallengeID: secondID, UserID: user.ID, SessionID: session.ID, Code: secondCode,
		}, verifier, now.Add(2*time.Minute)); err != nil {
			t.Fatalf("latest email change failed: %v", err)
		}
		updated, err := tdb.Store.GetUserByID(ctx, user.ID)
		if err != nil || updated.Email != "second-email@example.com" {
			t.Fatalf("email after latest change = %q, err=%v", updated.Email, err)
		}
		if _, err := tdb.Store.GetPendingPasswordResetByChallengeID(ctx, resetID); !errors.Is(err, store.ErrorPendingPasswordResetNotFound) {
			t.Fatalf("old-address reset survived email change: %v", err)
		}
		if err := svc.CompletePasswordResetChallenge(ctx, resetID, resetCode, verifier, now.Add(3*time.Minute)); !errors.Is(err, ErrPasswordResetUnavailable) {
			t.Fatalf("old-address reset completion error = %v", err)
		}
		assertPasswordHash(t, ctx, tdb, user.ID, "old-hash")
	})
}

func TestEmailChangeRejectsStaleExpectedEmailWithoutConsumingCode(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
		user := createPendingActionTestUser(t, ctx, tdb, "expected-email@example.com")
		session := createEmailChangeTestSession(t, ctx, tdb, user, now)
		svc, verifier := newPendingActionTestServices(tdb)
		challengeID, code := createEmailChangeTestChallenge(t, ctx, tdb, svc, verifier, user, session.ID, "stale-target@example.com", now)

		updated, err := tdb.Store.UpdateUserEmailIfCurrent(ctx, user.ID, user.Email, "newer-email@example.com")
		if err != nil || !updated {
			t.Fatalf("simulate newer email mutation: updated=%t err=%v", updated, err)
		}
		if err := svc.CompleteEmailChangeChallenge(ctx, CompleteEmailChangeChallengeInput{
			ChallengeID: challengeID, UserID: user.ID, SessionID: session.ID, Code: code,
		}, verifier, now.Add(time.Minute)); !errors.Is(err, ErrEmailChangeNotAuthorized) {
			t.Fatalf("stale email change completion error = %v", err)
		}
		current, err := tdb.Store.GetUserByID(ctx, user.ID)
		if err != nil || current.Email != "newer-email@example.com" {
			t.Fatalf("stale action changed email to %q, err=%v", current.Email, err)
		}
		challenge, err := tdb.Store.GetChallengeByID(ctx, challengeID)
		if err != nil || challenge.ConsumedAt != nil {
			t.Fatalf("stale action consumed challenge: consumed_at=%v err=%v", challenge.ConsumedAt, err)
		}
	})
}

func TestPasswordResetLateFailureRollsBackMutationAndChallenge(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC)
	suffix := uuid.NewString()
	address := "atomic-reset-" + suffix + "@example.com"
	user := createPendingActionTestUser(t, ctx, tdb, address)
	organizationID := uuid.Nil
	cleanupCommittedChallengeTest(t, tdb, user.ID, &organizationID, address)

	session := createEmailChangeTestSession(t, ctx, tdb, user, now)
	organizationID = session.ActiveOrganizationID
	completionErr := errors.New("access-token revocation unavailable")
	revocationCache := &toggleFailureCache{err: completionErr}
	revocationCache.fail.Store(true)
	svc := New(Config{
		Store:                  tdb.Store,
		Tx:                     tdb.Tx,
		ChallengeTTL:           30 * time.Minute,
		MaxAttempts:            5,
		MaxResends:             3,
		AccessTokenRevocations: token.NewAccessTokenRevocations(revocationCache, time.Hour),
	})
	verifier := NewVerificationCodeService(tdb.Store, 10*time.Minute, []byte("01234567890123456789012345678901"))
	challengeID, code := createPasswordResetTestChallenge(t, ctx, tdb, svc, verifier, user, "new-hash", now)

	if err := svc.CompletePasswordResetChallenge(ctx, challengeID, code, verifier, now.Add(time.Minute)); !errors.Is(err, completionErr) {
		t.Fatalf("completion error = %v, want %v", err, completionErr)
	}
	assertPasswordHash(t, ctx, tdb, user.ID, "old-hash")
	storedSession, err := tdb.Store.GetSessionByID(ctx, session.ID)
	if err != nil || storedSession.RevokedAt != nil {
		t.Fatalf("session after failed completion = (%+v, %v), want active", storedSession, err)
	}
	challenge, err := tdb.Store.GetChallengeByID(ctx, challengeID)
	if err != nil || challenge.ConsumedAt != nil {
		t.Fatalf("challenge after failed completion = (%+v, %v), want unconsumed", challenge, err)
	}
	if _, err := tdb.Store.GetPendingPasswordResetByChallengeID(ctx, challengeID); err != nil {
		t.Fatalf("pending reset after failed completion: %v", err)
	}
	if got := countCommittedEmailJobs(t, tdb, address, domain.EmailTemplatePasswordChanged); got != 0 {
		t.Fatalf("password-changed jobs after failed completion = %d, want 0", got)
	}

	revocationCache.fail.Store(false)
	if err := svc.CompletePasswordResetChallenge(ctx, challengeID, code, verifier, now.Add(2*time.Minute)); err != nil {
		t.Fatalf("retry completion: %v", err)
	}
	assertPasswordHash(t, ctx, tdb, user.ID, "new-hash")
	storedSession, err = tdb.Store.GetSessionByID(ctx, session.ID)
	if err != nil || storedSession.RevokedAt == nil {
		t.Fatalf("session after successful retry = (%+v, %v), want revoked", storedSession, err)
	}
	challenge, err = tdb.Store.GetChallengeByID(ctx, challengeID)
	if err != nil || challenge.ConsumedAt == nil {
		t.Fatalf("challenge after successful retry = (%+v, %v), want consumed", challenge, err)
	}
	if _, err := tdb.Store.GetPendingPasswordResetByChallengeID(ctx, challengeID); !errors.Is(err, store.ErrorPendingPasswordResetNotFound) {
		t.Fatalf("pending reset after successful retry: %v", err)
	}
	if got := countCommittedEmailJobs(t, tdb, address, domain.EmailTemplatePasswordChanged); got != 1 {
		t.Fatalf("password-changed jobs after successful retry = %d, want 1", got)
	}
}

func TestEmailChangeLateFailureRollsBackMutationAndChallenge(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 16, 11, 0, 0, 0, time.UTC)
	suffix := uuid.NewString()
	oldEmail := "atomic-email-old-" + suffix + "@example.com"
	newEmail := "atomic-email-new-" + suffix + "@example.com"
	user := createPendingActionTestUser(t, ctx, tdb, oldEmail)
	organizationID := uuid.Nil
	cleanupCommittedChallengeTest(t, tdb, user.ID, &organizationID, oldEmail, newEmail)
	session := createEmailChangeTestSession(t, ctx, tdb, user, now)
	organizationID = session.ActiveOrganizationID

	completionErr := errors.New("webhook enqueue unavailable")
	failCompletion := true
	pub := challengePublisherFunc(func(context.Context, webhook.Envelope) error {
		if failCompletion {
			return completionErr
		}
		return nil
	})
	svc := New(Config{
		Store:            tdb.Store,
		Tx:               tdb.Tx,
		ChallengeTTL:     30 * time.Minute,
		MaxAttempts:      5,
		MaxResends:       3,
		WebhookPublisher: pub,
	})
	verifier := NewVerificationCodeService(tdb.Store, 10*time.Minute, []byte("01234567890123456789012345678901"))
	challengeID, code := createEmailChangeTestChallenge(t, ctx, tdb, svc, verifier, user, session.ID, newEmail, now)
	input := CompleteEmailChangeChallengeInput{
		ChallengeID: challengeID,
		UserID:      user.ID,
		SessionID:   session.ID,
		Code:        code,
	}

	if err := svc.CompleteEmailChangeChallenge(ctx, input, verifier, now.Add(time.Minute)); !errors.Is(err, completionErr) {
		t.Fatalf("completion error = %v, want %v", err, completionErr)
	}
	current, err := tdb.Store.GetUserByID(ctx, user.ID)
	if err != nil || current.Email != oldEmail {
		t.Fatalf("user after failed completion = (%+v, %v), want email %q", current, err, oldEmail)
	}
	challenge, err := tdb.Store.GetChallengeByID(ctx, challengeID)
	if err != nil || challenge.ConsumedAt != nil {
		t.Fatalf("challenge after failed completion = (%+v, %v), want unconsumed", challenge, err)
	}
	if _, err := tdb.Store.GetPendingEmailChangeByChallengeID(ctx, challengeID); err != nil {
		t.Fatalf("pending email change after failed completion: %v", err)
	}
	if got := countCommittedEmailJobs(t, tdb, oldEmail, domain.EmailTemplateEmailChangedOldAddress); got != 0 {
		t.Fatalf("old-address jobs after failed completion = %d, want 0", got)
	}
	if got := countCommittedEmailJobs(t, tdb, newEmail, domain.EmailTemplateEmailChangedNewAddress); got != 0 {
		t.Fatalf("new-address jobs after failed completion = %d, want 0", got)
	}

	failCompletion = false
	if err := svc.CompleteEmailChangeChallenge(ctx, input, verifier, now.Add(2*time.Minute)); err != nil {
		t.Fatalf("retry completion: %v", err)
	}
	current, err = tdb.Store.GetUserByID(ctx, user.ID)
	if err != nil || current.Email != newEmail {
		t.Fatalf("user after successful retry = (%+v, %v), want email %q", current, err, newEmail)
	}
	challenge, err = tdb.Store.GetChallengeByID(ctx, challengeID)
	if err != nil || challenge.ConsumedAt == nil {
		t.Fatalf("challenge after successful retry = (%+v, %v), want consumed", challenge, err)
	}
	if _, err := tdb.Store.GetPendingEmailChangeByChallengeID(ctx, challengeID); !errors.Is(err, store.ErrorPendingEmailChangeNotFound) {
		t.Fatalf("pending email change after successful retry: %v", err)
	}
	if got := countCommittedEmailJobs(t, tdb, oldEmail, domain.EmailTemplateEmailChangedOldAddress); got != 1 {
		t.Fatalf("old-address jobs after successful retry = %d, want 1", got)
	}
	if got := countCommittedEmailJobs(t, tdb, newEmail, domain.EmailTemplateEmailChangedNewAddress); got != 1 {
		t.Fatalf("new-address jobs after successful retry = %d, want 1", got)
	}
}

func TestConcurrentPasswordResetCompletionHasSingleWinner(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	address := "concurrent-reset-" + uuid.NewString() + "@example.com"
	user := createPendingActionTestUser(t, ctx, tdb, address)
	cleanupCommittedChallengeTest(t, tdb, user.ID, nil, address)
	svc, verifier := newPendingActionTestServices(tdb)
	challengeID, code := createPasswordResetTestChallenge(t, ctx, tdb, svc, verifier, user, "winner-hash", now)

	assertSingleConcurrentWinner(t, func(callCtx context.Context) error {
		return svc.CompletePasswordResetChallenge(callCtx, challengeID, code, verifier, now.Add(time.Minute))
	})

	assertPasswordHash(t, ctx, tdb, user.ID, "winner-hash")
	challenge, err := tdb.Store.GetChallengeByID(ctx, challengeID)
	if err != nil || challenge.ConsumedAt == nil {
		t.Fatalf("challenge after concurrent completion = (%+v, %v), want consumed", challenge, err)
	}
	if _, err := tdb.Store.GetPendingPasswordResetByChallengeID(ctx, challengeID); !errors.Is(err, store.ErrorPendingPasswordResetNotFound) {
		t.Fatalf("pending reset after concurrent completion: %v", err)
	}
	if got := countCommittedEmailJobs(t, tdb, address, domain.EmailTemplatePasswordChanged); got != 1 {
		t.Fatalf("password-changed jobs after concurrent completion = %d, want 1", got)
	}
}

func TestConcurrentEmailChangeCompletionHasSingleWinner(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 16, 13, 0, 0, 0, time.UTC)
	suffix := uuid.NewString()
	oldEmail := "concurrent-email-old-" + suffix + "@example.com"
	newEmail := "concurrent-email-new-" + suffix + "@example.com"
	user := createPendingActionTestUser(t, ctx, tdb, oldEmail)
	organizationID := uuid.Nil
	cleanupCommittedChallengeTest(t, tdb, user.ID, &organizationID, oldEmail, newEmail)
	session := createEmailChangeTestSession(t, ctx, tdb, user, now)
	organizationID = session.ActiveOrganizationID
	svc, verifier := newPendingActionTestServices(tdb)
	challengeID, code := createEmailChangeTestChallenge(t, ctx, tdb, svc, verifier, user, session.ID, newEmail, now)
	input := CompleteEmailChangeChallengeInput{
		ChallengeID: challengeID,
		UserID:      user.ID,
		SessionID:   session.ID,
		Code:        code,
	}

	assertSingleConcurrentWinner(t, func(callCtx context.Context) error {
		return svc.CompleteEmailChangeChallenge(callCtx, input, verifier, now.Add(time.Minute))
	})

	current, err := tdb.Store.GetUserByID(ctx, user.ID)
	if err != nil || current.Email != newEmail {
		t.Fatalf("user after concurrent completion = (%+v, %v), want email %q", current, err, newEmail)
	}
	challenge, err := tdb.Store.GetChallengeByID(ctx, challengeID)
	if err != nil || challenge.ConsumedAt == nil {
		t.Fatalf("challenge after concurrent completion = (%+v, %v), want consumed", challenge, err)
	}
	if _, err := tdb.Store.GetPendingEmailChangeByChallengeID(ctx, challengeID); !errors.Is(err, store.ErrorPendingEmailChangeNotFound) {
		t.Fatalf("pending email change after concurrent completion: %v", err)
	}
	if got := countCommittedEmailJobs(t, tdb, oldEmail, domain.EmailTemplateEmailChangedOldAddress); got != 1 {
		t.Fatalf("old-address jobs after concurrent completion = %d, want 1", got)
	}
	if got := countCommittedEmailJobs(t, tdb, newEmail, domain.EmailTemplateEmailChangedNewAddress); got != 1 {
		t.Fatalf("new-address jobs after concurrent completion = %d, want 1", got)
	}
}

func assertSingleConcurrentWinner(t *testing.T, complete func(context.Context) error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := make(chan struct{})
	results := make(chan error, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	for range 2 {
		go func() {
			ready.Done()
			<-start
			results <- complete(ctx)
		}()
	}
	ready.Wait()
	close(start)

	successes := 0
	consumed := 0
	for range 2 {
		select {
		case err := <-results:
			switch {
			case err == nil:
				successes++
			case errors.Is(err, ErrChallengeConsumed):
				consumed++
			default:
				t.Fatalf("concurrent completion error = %v", err)
			}
		case <-ctx.Done():
			t.Fatalf("concurrent completion timed out: %v", ctx.Err())
		}
	}
	if successes != 1 || consumed != 1 {
		t.Fatalf("concurrent results: successes=%d consumed=%d, want 1 each", successes, consumed)
	}
}

func cleanupCommittedChallengeTest(
	t *testing.T,
	tdb *testutil.TestDB,
	userID uuid.UUID,
	organizationID *uuid.UUID,
	emails ...string,
) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, address := range emails {
			_ = tdb.Store.DeleteUserEmailReferences(ctx, userID, address)
		}
		_ = tdb.Store.DeleteUser(ctx, userID)
		if organizationID != nil && *organizationID != uuid.Nil {
			_ = tdb.Store.DeleteOrganization(ctx, *organizationID)
		}
	})
}

func countCommittedEmailJobs(t *testing.T, tdb *testutil.TestDB, address string, template domain.EmailTemplate) int {
	t.Helper()
	var count int
	if err := tdb.Store.DB().QueryRowContext(context.Background(), `
		SELECT count(*)
		FROM authara.email_jobs
		WHERE lower(to_email) = lower($1) AND template = $2
	`, address, string(template)).Scan(&count); err != nil {
		t.Fatalf("count committed email jobs: %v", err)
	}
	return count
}

func newPendingActionTestServices(tdb *testutil.TestDB) (*Service, *VerificationCodeService) {
	svc := New(Config{Store: tdb.Store, Tx: tdb.Tx, ChallengeTTL: 30 * time.Minute, MaxAttempts: 5, MaxResends: 3})
	verifier := NewVerificationCodeService(tdb.Store, 10*time.Minute, []byte("01234567890123456789012345678901"))
	return svc, verifier
}

func createPendingActionTestUser(t *testing.T, ctx context.Context, tdb *testutil.TestDB, address string) domain.User {
	t.Helper()
	user, err := tdb.Store.CreateUser(ctx, domain.User{Email: address, Username: address})
	if err != nil {
		t.Fatal(err)
	}
	oldHash := "old-hash"
	if _, err := tdb.Store.CreateAuthProvider(ctx, domain.AuthProvider{
		UserID: user.ID, Provider: domain.ProviderPassword, PasswordHash: &oldHash,
	}); err != nil {
		t.Fatal(err)
	}
	return user
}

func createPasswordResetTestChallenge(
	t *testing.T,
	ctx context.Context,
	tdb *testutil.TestDB,
	svc *Service,
	verifier *VerificationCodeService,
	user domain.User,
	passwordHash string,
	now time.Time,
) (uuid.UUID, string) {
	t.Helper()
	challengeID, err := svc.CreatePasswordResetChallenge(ctx, CreatePasswordResetChallengeInput{
		UserID: user.ID, Email: user.Email, PasswordHash: passwordHash,
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	challenge, err := tdb.Store.GetChallengeByID(ctx, challengeID)
	if err != nil {
		t.Fatal(err)
	}
	code, err := verifier.GenerateCode(ctx, challenge, now)
	if err != nil {
		t.Fatal(err)
	}
	return challengeID, code
}

func assertPasswordHash(t *testing.T, ctx context.Context, tdb *testutil.TestDB, userID uuid.UUID, want string) {
	t.Helper()
	provider, err := tdb.Store.GetAuthProviderByMethodAndUserID(ctx, domain.ProviderPassword, userID)
	if err != nil || provider.PasswordHash == nil || *provider.PasswordHash != want {
		t.Fatalf("password hash = %v, err=%v, want %q", provider.PasswordHash, err, want)
	}
}
