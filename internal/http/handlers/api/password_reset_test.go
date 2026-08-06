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
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/session/token"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func TestPasswordResetChallengeWorksWhenOptionalChallengesDisabled(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		h := newAPIChallengeTestHandler(t, tdb)
		h.ChallengeEnabled = false
		oldHash, err := auth.Hash("old-password123")
		if err != nil {
			t.Fatalf("Hash failed: %v", err)
		}
		user, err := h.Auth.Signup(ctx, auth.SignupInput{
			Provider:     domain.ProviderPassword,
			Email:        "reset-api@example.com",
			PasswordHash: oldHash,
		})
		if err != nil {
			t.Fatalf("create user: %v", err)
		}
		if _, _, err := h.Session.CreateSession(ctx, user.ID, token.AudienceApp, "reset-test", time.Now().UTC()); err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}

		startReq := apiJSONRequest(ctx, http.MethodPost, "/auth/api/v1/password-reset/challenges", `{"email":" RESET-API@example.com ","new_password":"new-password123"}`)
		startRR := httptest.NewRecorder()
		startResp, err := h.StartPasswordResetChallenge(contractCtx(ctx, startReq), contract.StartPasswordResetChallengeRequestObject{
			Body: &contract.PasswordResetRequest{
				Email:       openapi_types.Email(" RESET-API@example.com "),
				NewPassword: "new-password123",
			},
		})
		if err != nil {
			t.Fatalf("StartPasswordResetChallenge failed: %v", err)
		}
		writeContractResponse(t, startRR, startResp)
		if startRR.Code != http.StatusAccepted {
			t.Fatalf("expected start status %d, got %d body=%s", http.StatusAccepted, startRR.Code, startRR.Body.String())
		}

		challengeID := decodePasswordResetChallengeID(t, startRR.Body.Bytes())
		row, err := tdb.Store.GetChallengeByID(ctx, challengeID)
		if err != nil {
			t.Fatalf("GetChallengeByID failed: %v", err)
		}
		code, err := h.Verification.GenerateCode(ctx, row, time.Now().UTC())
		if err != nil {
			t.Fatalf("GenerateCode failed: %v", err)
		}

		verifyReq := apiJSONRequest(ctx, http.MethodPost, "/auth/api/v1/password-reset/challenges/verify", "")
		verifyRR := httptest.NewRecorder()
		verifyResp, err := h.VerifyPasswordResetChallenge(contractCtx(ctx, verifyReq), contract.VerifyPasswordResetChallengeRequestObject{
			Body: &contract.PasswordResetChallengeVerification{ChallengeId: challengeID, Code: code},
		})
		if err != nil {
			t.Fatalf("VerifyPasswordResetChallenge failed: %v", err)
		}
		writeContractResponse(t, verifyRR, verifyResp)
		if verifyRR.Code != http.StatusNoContent || verifyRR.Body.Len() != 0 {
			t.Fatalf("expected verify status %d, got %d body=%s", http.StatusNoContent, verifyRR.Code, verifyRR.Body.String())
		}

		if _, err := h.Auth.Login(ctx, auth.LoginInput{Provider: domain.ProviderPassword, Email: user.Email, Password: "new-password123"}); err != nil {
			t.Fatalf("new password login failed: %v", err)
		}
		if _, err := h.Auth.Login(ctx, auth.LoginInput{Provider: domain.ProviderPassword, Email: user.Email, Password: "old-password123"}); !errors.Is(err, auth.ErrInvalidCredentials) {
			t.Fatalf("expected old password to fail, got %v", err)
		}
		sessions, err := tdb.Store.ListSessionsByUserID(ctx, user.ID)
		if err != nil {
			t.Fatalf("ListSessionsByUserID failed: %v", err)
		}
		if len(sessions) != 1 || sessions[0].RevokedAt == nil {
			t.Fatalf("expected existing session to be revoked, got %+v", sessions)
		}
		if _, err := tdb.Store.GetPendingPasswordResetByChallengeID(ctx, challengeID); !errors.Is(err, store.ErrorPendingPasswordResetNotFound) {
			t.Fatalf("expected pending reset to be deleted, got %v", err)
		}
	})
}

func TestPasswordResetChallengeUnknownEmailIsOpaque(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		h := newAPIChallengeTestHandler(t, tdb)
		req := apiJSONRequest(ctx, http.MethodPost, "/auth/api/v1/password-reset/challenges", `{"email":"missing-reset@example.com","new_password":"new-password123"}`)
		rr := httptest.NewRecorder()
		resp, err := h.StartPasswordResetChallenge(contractCtx(ctx, req), contract.StartPasswordResetChallengeRequestObject{
			Body: &contract.PasswordResetRequest{
				Email:       openapi_types.Email("missing-reset@example.com"),
				NewPassword: "new-password123",
			},
		})
		if err != nil {
			t.Fatalf("StartPasswordResetChallenge failed: %v", err)
		}
		writeContractResponse(t, rr, resp)
		if rr.Code != http.StatusAccepted {
			t.Fatalf("expected status %d, got %d body=%s", http.StatusAccepted, rr.Code, rr.Body.String())
		}

		challengeID := decodePasswordResetChallengeID(t, rr.Body.Bytes())
		row, err := tdb.Store.GetChallengeByID(ctx, challengeID)
		if err != nil {
			t.Fatalf("GetChallengeByID failed: %v", err)
		}
		if row.MaxResends != 0 {
			t.Fatalf("expected opaque challenge, max_resends=%d", row.MaxResends)
		}
		if _, err := tdb.Store.GetPendingPasswordResetByChallengeID(ctx, challengeID); !errors.Is(err, store.ErrorPendingPasswordResetNotFound) {
			t.Fatalf("expected no pending reset, got %v", err)
		}
	})
}

func decodePasswordResetChallengeID(t *testing.T, body []byte) uuid.UUID {
	t.Helper()

	var got contract.ChallengeReference
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode challenge response: %v", err)
	}
	return got.ChallengeId
}
