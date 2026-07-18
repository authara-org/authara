package challenge

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/testutil"
)

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
	})
}
