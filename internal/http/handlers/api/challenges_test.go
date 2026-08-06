package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/challenge"
	"github.com/authara-org/authara/internal/domain"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/organization"
	"github.com/authara-org/authara/internal/ratelimiter"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/google/uuid"
)

func TestSignupChallengeVerificationCreatesSession(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		h := newAPIChallengeTestHandler(t, tdb)

		signupReq := apiJSONRequest(
			ctx,
			http.MethodPost,
			"/auth/api/v1/signup/challenges",
			`{"email":"challenge-api@example.com","password":"password123"}`,
		)
		signupRR := httptest.NewRecorder()
		signupResp, err := h.StartSignupChallenge(contractCtx(ctx, signupReq), contract.StartSignupChallengeRequestObject{
			Body: signupRequest("challenge-api@example.com", "password123", ""),
		})
		if err != nil {
			t.Fatalf("StartSignupChallenge failed: %v", err)
		}
		writeContractResponse(t, signupRR, signupResp)

		if signupRR.Code != http.StatusAccepted {
			t.Fatalf("expected signup status %d, got %d body=%s", http.StatusAccepted, signupRR.Code, signupRR.Body.String())
		}
		if hasCookie(signupRR.Result().Cookies(), "authara_access") || hasCookie(signupRR.Result().Cookies(), "authara_refresh") {
			t.Fatal("expected challenge start not to create a session")
		}

		challengeID := decodeChallengeID(t, signupRR.Body.Bytes())
		row, err := tdb.Store.GetChallengeByID(ctx, challengeID)
		if err != nil {
			t.Fatalf("GetChallengeByID failed: %v", err)
		}
		if _, err := tdb.Store.GetPendingSignupActionByChallengeID(ctx, challengeID); err != nil {
			t.Fatalf("expected pending signup action: %v", err)
		}
		code, err := h.Verification.GenerateCode(ctx, row, time.Now().UTC())
		if err != nil {
			t.Fatalf("GenerateCode failed: %v", err)
		}

		verifyReq := apiJSONRequest(
			ctx,
			http.MethodPost,
			"/auth/api/v1/signup/challenges/verify",
			"",
		)
		verifyRR := httptest.NewRecorder()
		verifyResp, err := h.VerifySignupChallenge(contractCtx(ctx, verifyReq), contract.VerifySignupChallengeRequestObject{
			Body: &contract.SignupChallengeVerification{ChallengeId: challengeID, Code: code},
		})
		if err != nil {
			t.Fatalf("VerifySignupChallenge failed: %v", err)
		}
		writeContractResponse(t, verifyRR, verifyResp)

		if verifyRR.Code != http.StatusCreated {
			t.Fatalf("expected verify status %d, got %d body=%s", http.StatusCreated, verifyRR.Code, verifyRR.Body.String())
		}
		if !hasCookie(verifyRR.Result().Cookies(), "authara_access") || !hasCookie(verifyRR.Result().Cookies(), "authara_refresh") {
			t.Fatal("expected verification to create session cookies")
		}
		assertResponseTokens(t, verifyRR.Body.Bytes())
		if _, err := h.Auth.GetUserByEmail(ctx, "challenge-api@example.com"); err != nil {
			t.Fatalf("expected verified user: %v", err)
		}
	})
}

func TestSignupChallengeExistingEmailIsOpaque(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		h := newAPIChallengeTestHandler(t, tdb)
		hash, err := auth.Hash("password123")
		if err != nil {
			t.Fatalf("Hash failed: %v", err)
		}
		if _, err := h.Auth.Signup(ctx, auth.SignupInput{
			Provider:     domain.ProviderPassword,
			Email:        "opaque-api@example.com",
			PasswordHash: hash,
		}); err != nil {
			t.Fatalf("create existing user: %v", err)
		}

		req := apiJSONRequest(
			ctx,
			http.MethodPost,
			"/auth/api/v1/signup/challenges",
			`{"email":"opaque-api@example.com","password":"password123"}`,
		)
		rr := httptest.NewRecorder()
		resp, err := h.StartSignupChallenge(contractCtx(ctx, req), contract.StartSignupChallengeRequestObject{
			Body: signupRequest("opaque-api@example.com", "password123", ""),
		})
		if err != nil {
			t.Fatalf("StartSignupChallenge failed: %v", err)
		}
		writeContractResponse(t, rr, resp)

		if rr.Code != http.StatusAccepted {
			t.Fatalf("expected status %d, got %d body=%s", http.StatusAccepted, rr.Code, rr.Body.String())
		}
		challengeID := decodeChallengeID(t, rr.Body.Bytes())
		row, err := tdb.Store.GetChallengeByID(ctx, challengeID)
		if err != nil {
			t.Fatalf("GetChallengeByID failed: %v", err)
		}
		if row.MaxResends != 0 {
			t.Fatalf("expected opaque challenge, max_resends=%d", row.MaxResends)
		}
		if _, err := tdb.Store.GetPendingSignupActionByChallengeID(ctx, challengeID); !errors.Is(err, store.ErrorPendingSignupActionNotFound) {
			t.Fatalf("expected no pending signup action, got %v", err)
		}
	})
}

func TestSignupChallengeWithInvitationCodeRejectsWrongEmail(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		h := newAPIChallengeTestHandler(t, tdb)
		owner, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "challenge-invite-owner@example.com",
			Username: "challenge-invite-owner",
		})
		if err != nil {
			t.Fatalf("CreateUser owner failed: %v", err)
		}
		org, _, err := tdb.Store.EnsureOrganizationForUser(ctx, owner.ID, owner.Username, domain.OrganizationKindTeam)
		if err != nil {
			t.Fatalf("EnsureOrganizationForUser owner failed: %v", err)
		}
		invite, err := h.Organizations.CreateInvitation(ctx, organization.CreateInvitationInput{
			OrganizationID: org.ID,
			ActorUserID:    owner.ID,
			Email:          "challenge-right-invited@example.com",
			Now:            time.Now().UTC(),
		})
		if err != nil {
			t.Fatalf("CreateInvitation failed: %v", err)
		}

		req := apiJSONRequest(
			ctx,
			http.MethodPost,
			"/auth/api/v1/signup/challenges",
			`{"email":"challenge-wrong-invited@example.com","password":"password123","invitation_code":"`+invite.RawToken+`"}`,
		)
		rr := httptest.NewRecorder()
		resp, err := h.StartSignupChallenge(contractCtx(ctx, req), contract.StartSignupChallengeRequestObject{
			Body: signupRequest("challenge-wrong-invited@example.com", "password123", invite.RawToken),
		})
		if err != nil {
			t.Fatalf("StartSignupChallenge failed: %v", err)
		}
		writeContractResponse(t, rr, resp)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected signup status %d, got %d body=%s", http.StatusBadRequest, rr.Code, rr.Body.String())
		}
		assertErrorMessage(t, rr.Body.Bytes(), "Invitation code does not match this email.")
	})
}

func TestChallengeResendKeepsChallengeStateOpaque(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		h := newAPIChallengeTestHandler(t, tdb)
		h.ChallengeEnabled = false
		now := time.Now().UTC()
		realID, err := h.Challenge.CreateSignupChallenge(ctx, challenge.CreateSignupChallengeInput{
			Email:        "resend-real@example.com",
			PasswordHash: "hash",
		}, now)
		if err != nil {
			t.Fatalf("CreateSignupChallenge failed: %v", err)
		}
		opaqueID, err := h.Challenge.CreateOpaqueChallenge(
			ctx,
			now,
			domain.ChallengePurposeSignup,
			"resend-opaque@example.com",
		)
		if err != nil {
			t.Fatalf("CreateOpaqueChallenge failed: %v", err)
		}

		for _, challengeID := range []uuid.UUID{realID, opaqueID, uuid.New()} {
			req := apiJSONRequest(
				ctx,
				http.MethodPost,
				"/auth/api/v1/challenges/resend",
				"",
			)
			rr := httptest.NewRecorder()
			resp, err := h.ResendChallenge(contractCtx(ctx, req), contract.ResendChallengeRequestObject{
				Body: &contract.ChallengeReference{ChallengeId: challengeID},
			})
			if err != nil {
				t.Fatalf("ResendChallenge failed: %v", err)
			}
			writeContractResponse(t, rr, resp)

			if rr.Code != http.StatusNoContent || rr.Body.Len() != 0 {
				t.Fatalf("expected opaque 204 response for %s, got %d body=%s", challengeID, rr.Code, rr.Body.String())
			}
		}
	})
}

func TestSignupChallengeVerifyRejectsInvalidCodeShape(t *testing.T) {
	h := &APIHandler{ChallengeEnabled: true, Challenge: &challenge.Service{}, Verification: &challenge.VerificationCodeService{}}
	req := httptest.NewRequest(
		http.MethodPost,
		"/auth/api/v1/signup/challenges/verify",
		strings.NewReader(`{"challenge_id":"`+uuid.NewString()+`","code":"abcdef"}`),
	)
	rr := httptest.NewRecorder()

	challengeID := uuid.New()
	resp, err := h.VerifySignupChallenge(contractCtx(req.Context(), req), contract.VerifySignupChallengeRequestObject{
		Body: &contract.SignupChallengeVerification{ChallengeId: challengeID, Code: "abcdef"},
	})
	if err != nil {
		t.Fatalf("VerifySignupChallenge failed: %v", err)
	}
	writeContractResponse(t, rr, resp)

	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "invalid_request") {
		t.Fatalf("expected invalid_request, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestSignupRoutesReturnNotFoundWhenUnavailable(t *testing.T) {
	tests := []struct {
		name    string
		handler *APIHandler
		target  string
		handle  func(*APIHandler, context.Context, *http.Request) (any, error)
	}{
		{
			name:    "direct signup with challenges enabled",
			handler: &APIHandler{ChallengeEnabled: true},
			target:  "/auth/api/v1/signup/direct",
			handle: func(h *APIHandler, ctx context.Context, r *http.Request) (any, error) {
				return h.SignupDirect(ctx, contract.SignupDirectRequestObject{Body: signupRequest("", "", "")})
			},
		},
		{
			name:    "challenge signup with challenges disabled",
			handler: &APIHandler{},
			target:  "/auth/api/v1/signup/challenges",
			handle: func(h *APIHandler, ctx context.Context, r *http.Request) (any, error) {
				return h.StartSignupChallenge(ctx, contract.StartSignupChallengeRequestObject{Body: signupRequest("", "", "")})
			},
		},
		{
			name:    "challenge verification with challenges disabled",
			handler: &APIHandler{},
			target:  "/auth/api/v1/signup/challenges/verify",
			handle: func(h *APIHandler, ctx context.Context, r *http.Request) (any, error) {
				return h.VerifySignupChallenge(ctx, contract.VerifySignupChallengeRequestObject{Body: &contract.SignupChallengeVerification{ChallengeId: uuid.New(), Code: "000000"}})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.target, strings.NewReader(`{}`))
			rr := httptest.NewRecorder()

			resp, err := tt.handle(tt.handler, contractCtx(req.Context(), req), req)
			if err != nil {
				t.Fatalf("handler failed: %v", err)
			}
			writeContractResponse(t, rr, resp)

			if rr.Code != http.StatusNotFound || !strings.Contains(rr.Body.String(), `"code":"not_found"`) {
				t.Fatalf("expected not_found, got %d body=%s", rr.Code, rr.Body.String())
			}
		})
	}
}

func newAPIChallengeTestHandler(t *testing.T, tdb *testutil.TestDB) *APIHandler {
	t.Helper()

	organizations := organization.New(organization.Config{
		Store: tdb.Store,
		Tx:    tdb.Tx,
		Mode:  organization.OrgModeSingle,
	})
	challengeService := challenge.New(challenge.Config{
		Store:             tdb.Store,
		Tx:                tdb.Tx,
		ChallengeTTL:      30 * time.Minute,
		MaxAttempts:       5,
		MaxResends:        3,
		MinResendInterval: 0,
	})
	verification := challenge.NewVerificationCodeService(
		tdb.Store,
		10*time.Minute,
		[]byte("01234567890123456789012345678901"),
	)

	return &APIHandler{
		Auth: auth.New(auth.Config{
			Store:         tdb.Store,
			Tx:            tdb.Tx,
			Organizations: organizations,
		}),
		Organizations:    organizations,
		Session:          newAPIHandlerTestSessionService(t, tdb),
		Challenge:        challengeService,
		Verification:     verification,
		Limiter:          ratelimiter.NewInMemoryLimiter(ratelimiter.LimiterConfig{}),
		ChallengeEnabled: true,
		AccessTTL:        time.Minute,
		RefreshTTL:       time.Hour,
	}
}

func decodeChallengeID(t *testing.T, body []byte) uuid.UUID {
	t.Helper()

	var got contract.SignupChallenge
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode challenge response: %v", err)
	}
	return got.ChallengeId
}
