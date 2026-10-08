package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/testutil"
)

func TestSetCurrentUserPasswordUsesAuthenticatedUser(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "current-user-password@example.com",
			Username: "current-user-password",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}
		org, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username)
		if err != nil {
			t.Fatalf("EnsureDefaultOrganizationForUser failed: %v", err)
		}
		session, err := tdb.Store.CreateSession(ctx, domain.Session{
			UserID:               user.ID,
			ActiveOrganizationID: org.ID,
			ExpiresAt:            time.Now().UTC().Add(time.Hour),
			UserAgent:            "current-user-password-test",
		})
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}
		challenge, err := tdb.Store.CreateChallenge(ctx, domain.Challenge{
			Purpose:     domain.ChallengePurposePasswordReset,
			Email:       user.Email,
			ExpiresAt:   time.Now().UTC().Add(time.Hour),
			MaxAttempts: 5,
			MaxResends:  3,
		})
		if err != nil {
			t.Fatalf("CreateChallenge failed: %v", err)
		}
		if _, err := tdb.Store.CreatePendingPasswordReset(ctx, domain.PendingPasswordReset{
			ChallengeID:  challenge.ID,
			UserID:       user.ID,
			PasswordHash: "stale-password-hash",
		}); err != nil {
			t.Fatalf("CreatePendingPasswordReset failed: %v", err)
		}

		h := &APIHandler{Auth: auth.New(auth.Config{Store: tdb.Store, Tx: tdb.Tx})}
		reqCtx := httpctx.WithUserID(ctx, user.ID)
		rr := httptest.NewRecorder()
		resp, err := h.SetCurrentUserPassword(reqCtx, contract.SetCurrentUserPasswordRequestObject{
			Body: &contract.SetPasswordRequest{Password: "new-password123"},
		})
		if err != nil {
			t.Fatalf("SetCurrentUserPassword failed: %v", err)
		}
		writeContractResponse(t, rr, resp)
		if rr.Code != http.StatusNoContent || rr.Body.Len() != 0 {
			t.Fatalf("expected status %d, got %d body=%s", http.StatusNoContent, rr.Code, rr.Body.String())
		}

		provider, err := tdb.Store.GetAuthProviderByMethodAndUserID(ctx, domain.ProviderPassword, user.ID)
		if err != nil {
			t.Fatalf("GetAuthProviderByMethodAndUserID failed: %v", err)
		}
		valid, err := auth.Verify("new-password123", *provider.PasswordHash)
		if err != nil || !valid {
			t.Fatalf("expected new password to verify, valid=%t err=%v", valid, err)
		}
		storedSession, err := tdb.Store.GetSessionByID(ctx, session.ID)
		if err != nil {
			t.Fatalf("GetSessionByID failed: %v", err)
		}
		if storedSession.RevokedAt == nil {
			t.Fatal("expected existing session to be revoked")
		}
		if _, err := tdb.Store.GetPendingPasswordResetByChallengeID(ctx, challenge.ID); !errors.Is(err, store.ErrorPendingPasswordResetNotFound) {
			t.Fatalf("expected pending password reset to be deleted, got %v", err)
		}
	})
}

func TestSetCurrentUserPasswordRejectsExistingPassword(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "existing-current-user-password@example.com",
			Username: "existing-current-user-password",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}
		org, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username)
		if err != nil {
			t.Fatalf("EnsureDefaultOrganizationForUser failed: %v", err)
		}
		session, err := tdb.Store.CreateSession(ctx, domain.Session{
			UserID:               user.ID,
			ActiveOrganizationID: org.ID,
			ExpiresAt:            time.Now().UTC().Add(time.Hour),
			UserAgent:            "existing-current-user-password-test",
		})
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}

		currentPasswordHash, err := auth.Hash("current-password123")
		if err != nil {
			t.Fatalf("Hash failed: %v", err)
		}
		if _, err := tdb.Store.CreateAuthProvider(ctx, domain.AuthProvider{
			UserID:       user.ID,
			Provider:     domain.ProviderPassword,
			PasswordHash: &currentPasswordHash,
		}); err != nil {
			t.Fatalf("CreateAuthProvider failed: %v", err)
		}

		challenge, err := tdb.Store.CreateChallenge(ctx, domain.Challenge{
			Purpose:     domain.ChallengePurposePasswordReset,
			Email:       user.Email,
			ExpiresAt:   time.Now().UTC().Add(time.Hour),
			MaxAttempts: 5,
			MaxResends:  3,
		})
		if err != nil {
			t.Fatalf("CreateChallenge failed: %v", err)
		}
		if _, err := tdb.Store.CreatePendingPasswordReset(ctx, domain.PendingPasswordReset{
			ChallengeID:  challenge.ID,
			UserID:       user.ID,
			PasswordHash: "stale-password-hash",
		}); err != nil {
			t.Fatalf("CreatePendingPasswordReset failed: %v", err)
		}

		h := &APIHandler{Auth: auth.New(auth.Config{Store: tdb.Store, Tx: tdb.Tx})}
		reqCtx := httpctx.WithUserID(ctx, user.ID)
		rr := httptest.NewRecorder()
		resp, err := h.SetCurrentUserPassword(reqCtx, contract.SetCurrentUserPasswordRequestObject{
			Body: &contract.SetPasswordRequest{Password: "replacement-password123"},
		})
		if err != nil {
			t.Fatalf("SetCurrentUserPassword failed: %v", err)
		}
		writeContractResponse(t, rr, resp)
		if rr.Code != http.StatusConflict {
			t.Fatalf("expected status %d, got %d body=%s", http.StatusConflict, rr.Code, rr.Body.String())
		}
		var body struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode error response: %v", err)
		}
		if body.Error.Code != "password_already_exists" {
			t.Fatalf("expected password_already_exists, got %q", body.Error.Code)
		}

		provider, err := tdb.Store.GetAuthProviderByMethodAndUserID(ctx, domain.ProviderPassword, user.ID)
		if err != nil {
			t.Fatalf("GetAuthProviderByMethodAndUserID failed: %v", err)
		}
		valid, err := auth.Verify("current-password123", *provider.PasswordHash)
		if err != nil || !valid {
			t.Fatalf("expected current password to remain valid, valid=%t err=%v", valid, err)
		}
		valid, err = auth.Verify("replacement-password123", *provider.PasswordHash)
		if err != nil {
			t.Fatalf("Verify replacement password failed: %v", err)
		}
		if valid {
			t.Fatal("replacement password unexpectedly became valid")
		}

		storedSession, err := tdb.Store.GetSessionByID(ctx, session.ID)
		if err != nil {
			t.Fatalf("GetSessionByID failed: %v", err)
		}
		if storedSession.RevokedAt != nil {
			t.Fatal("rejected password replacement revoked the existing session")
		}
		if _, err := tdb.Store.GetPendingPasswordResetByChallengeID(ctx, challenge.ID); err != nil {
			t.Fatalf("rejected password replacement removed the pending reset: %v", err)
		}
		if got := testutil.CountEmailJobs(t, ctx, user.Email, domain.EmailTemplatePasswordChanged); got != 0 {
			t.Fatalf("rejected password replacement queued %d password-changed emails, want 0", got)
		}
	})
}

func TestSetCurrentUserPasswordRequiresAuthenticatedUser(t *testing.T) {
	h := &APIHandler{}
	rr := httptest.NewRecorder()
	resp, err := h.SetCurrentUserPassword(context.Background(), contract.SetCurrentUserPasswordRequestObject{
		Body: &contract.SetPasswordRequest{Password: "new-password123"},
	})
	if err != nil {
		t.Fatalf("SetCurrentUserPassword failed: %v", err)
	}
	writeContractResponse(t, rr, resp)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusUnauthorized, rr.Code, rr.Body.String())
	}
}
