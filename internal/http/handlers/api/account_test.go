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
	"github.com/authara-org/authara/internal/testutil"
)

func TestCurrentAccountReadAndPasswordMutations(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user, err := tdb.Store.CreateUser(ctx, domain.User{Email: "account-api@example.com", Username: "account-api"})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}
		org, _, err := tdb.Store.EnsureDefaultOrganizationForUser(ctx, user.ID, user.Username)
		if err != nil {
			t.Fatalf("EnsureDefaultOrganizationForUser failed: %v", err)
		}
		now := time.Now().UTC()
		current, err := tdb.Store.CreateSession(ctx, domain.Session{UserID: user.ID, ActiveOrganizationID: org.ID, ExpiresAt: now.Add(time.Hour), UserAgent: "current"})
		if err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}
		if _, err := tdb.Store.CreateSession(ctx, domain.Session{UserID: user.ID, ActiveOrganizationID: org.ID, ExpiresAt: now.Add(time.Hour), UserAgent: "other"}); err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}

		h := &APIHandler{
			Auth:    auth.New(auth.Config{Store: tdb.Store, Tx: tdb.Tx}),
			Session: newAPIHandlerTestSessionService(t, tdb),
		}
		requestCtx := httpctx.WithSessionID(httpctx.WithUserID(ctx, user.ID), current.ID)
		rr := httptest.NewRecorder()
		resp, err := h.GetCurrentAccount(requestCtx, contract.GetCurrentAccountRequestObject{})
		if err != nil {
			t.Fatalf("GetCurrentAccount failed: %v", err)
		}
		writeContractResponse(t, rr, resp)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rr.Code, rr.Body.String())
		}
		var account contract.Account
		if err := json.Unmarshal(rr.Body.Bytes(), &account); err != nil {
			t.Fatalf("decode account: %v", err)
		}
		if len(account.Sessions) != 2 || !account.Sessions[0].Current {
			t.Fatalf("expected current session first, got %+v", account.Sessions)
		}

		rr = httptest.NewRecorder()
		addResp, err := h.AddCurrentUserPassword(requestCtx, contract.AddCurrentUserPasswordRequestObject{
			Body: &contract.SetPasswordRequest{Password: "new-password123"},
		})
		if err != nil {
			t.Fatalf("AddCurrentUserPassword failed: %v", err)
		}
		writeContractResponse(t, rr, addResp)
		if rr.Code != http.StatusNoContent {
			t.Fatalf("expected status %d, got %d body=%s", http.StatusNoContent, rr.Code, rr.Body.String())
		}

		rr = httptest.NewRecorder()
		changeResp, err := h.ChangeCurrentUserPassword(requestCtx, contract.ChangeCurrentUserPasswordRequestObject{
			Body: &contract.ChangePasswordRequest{CurrentPassword: "new-password123", NewPassword: "changed-password123"},
		})
		if err != nil {
			t.Fatalf("ChangeCurrentUserPassword failed: %v", err)
		}
		writeContractResponse(t, rr, changeResp)
		if rr.Code != http.StatusNoContent {
			t.Fatalf("expected status %d, got %d body=%s", http.StatusNoContent, rr.Code, rr.Body.String())
		}
		provider, err := tdb.Store.GetAuthProviderByMethodAndUserID(ctx, domain.ProviderPassword, user.ID)
		if err != nil {
			t.Fatalf("GetAuthProviderByMethodAndUserID failed: %v", err)
		}
		valid, err := auth.Verify("changed-password123", *provider.PasswordHash)
		if err != nil || !valid {
			t.Fatalf("expected changed password to verify, valid=%t err=%v", valid, err)
		}
	})
}
