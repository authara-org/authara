package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	contract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/passkey"
	"github.com/authara-org/authara/internal/ratelimiter"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/google/uuid"
)

func TestPasskeyAuthenticateOptionsReturnsJSON(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		h := &APIHandler{
			Passkeys: newAPIPasskeyTestService(t, tdb),
			Limiter:  ratelimiter.NewInMemoryLimiter(ratelimiter.LimiterConfig{}),
		}
		req := httptest.NewRequest(http.MethodPost, "/auth/api/v1/passkeys/authenticate/options", nil).WithContext(ctx)
		rr := httptest.NewRecorder()

		resp, err := h.BeginPasskeyAuthentication(contractCtx(ctx, req), contract.BeginPasskeyAuthenticationRequestObject{})
		if err != nil {
			t.Fatalf("BeginPasskeyAuthentication failed: %v", err)
		}
		writeContractResponse(t, rr, resp)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rr.Code, rr.Body.String())
		}
		var body struct {
			ChallengeID string `json:"challenge_id"`
			Options     struct {
				PublicKey struct {
					Challenge        string `json:"challenge"`
					UserVerification string `json:"userVerification"`
				} `json:"publicKey"`
			} `json:"options"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode options: %v", err)
		}
		if body.ChallengeID == "" || body.Options.PublicKey.Challenge == "" {
			t.Fatalf("expected challenge envelope, got %+v", body)
		}
		if body.Options.PublicKey.UserVerification != "required" {
			t.Fatalf("expected user verification required, got %q", body.Options.PublicKey.UserVerification)
		}
	})
}

func TestPasskeyRegisterOptionsBindsAuthenticatedUser(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user, err := tdb.Store.CreateUser(ctx, domain.User{
			Email:    "api-passkey-register@example.com",
			Username: "api-passkey-register",
		})
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}
		h := &APIHandler{Passkeys: newAPIPasskeyTestService(t, tdb)}
		req := httptest.NewRequest(http.MethodPost, "/auth/api/v1/passkeys/register/options", nil)
		req = req.WithContext(httpctx.WithUserID(ctx, user.ID))
		rr := httptest.NewRecorder()

		resp, err := h.BeginPasskeyRegistration(contractCtx(req.Context(), req), contract.BeginPasskeyRegistrationRequestObject{})
		if err != nil {
			t.Fatalf("BeginPasskeyRegistration failed: %v", err)
		}
		writeContractResponse(t, rr, resp)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rr.Code, rr.Body.String())
		}
		var body struct {
			ChallengeID string `json:"challenge_id"`
			Options     struct {
				PublicKey struct {
					User struct {
						ID string `json:"id"`
					} `json:"user"`
				} `json:"publicKey"`
			} `json:"options"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode options: %v", err)
		}
		if body.ChallengeID == "" || body.Options.PublicKey.User.ID == "" {
			t.Fatalf("expected registration envelope, got %+v", body)
		}

		challengeID, err := uuid.Parse(body.ChallengeID)
		if err != nil {
			t.Fatalf("parse challenge id: %v", err)
		}
		challenge, err := tdb.Store.GetWebAuthnChallengeByIDForUpdate(ctx, challengeID)
		if err != nil {
			t.Fatalf("load WebAuthn challenge: %v", err)
		}
		if challenge.UserID == nil || *challenge.UserID != user.ID {
			t.Fatalf("expected challenge bound to %s, got %v", user.ID, challenge.UserID)
		}
	})
}

func TestPasskeyFinishRejectsMalformedBody(t *testing.T) {
	tests := []struct {
		name    string
		handler func(*APIHandler, context.Context) (any, error)
		ctx     func(context.Context) context.Context
	}{
		{
			name: "authenticate",
			handler: func(h *APIHandler, ctx context.Context) (any, error) {
				return h.FinishPasskeyAuthentication(ctx, contract.FinishPasskeyAuthenticationRequestObject{})
			},
			ctx: func(ctx context.Context) context.Context { return ctx },
		},
		{
			name: "register",
			handler: func(h *APIHandler, ctx context.Context) (any, error) {
				return h.FinishPasskeyRegistration(ctx, contract.FinishPasskeyRegistrationRequestObject{})
			},
			ctx: func(ctx context.Context) context.Context {
				return httpctx.WithUserID(ctx, uuid.New())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &APIHandler{Passkeys: &passkey.Service{}}
			req := httptest.NewRequest(
				http.MethodPost,
				"/auth/api/v1/passkeys/"+tt.name+"/finish",
				strings.NewReader(`{"challenge_id":`),
			)
			req = req.WithContext(tt.ctx(req.Context()))
			rr := httptest.NewRecorder()

			resp, err := tt.handler(h, contractCtx(req.Context(), req))
			if err != nil {
				t.Fatalf("handler failed: %v", err)
			}
			writeContractResponse(t, rr, resp)

			if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "invalid_request") {
				t.Fatalf("expected invalid_request, got %d body=%s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestPasskeyLoginLimiterBucketsAreIndependent(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		h := &APIHandler{
			Passkeys: newAPIPasskeyTestService(t, tdb),
			Limiter: ratelimiter.NewInMemoryLimiter(ratelimiter.LimiterConfig{
				PasskeyLoginIPLimit:  1,
				PasskeyLoginIPWindow: time.Hour,
			}),
		}

		optionsReq := httptest.NewRequest(http.MethodPost, "/auth/api/v1/passkeys/authenticate/options", nil).WithContext(ctx)
		optionsRR := httptest.NewRecorder()
		optionsResp, err := h.BeginPasskeyAuthentication(contractCtx(ctx, optionsReq), contract.BeginPasskeyAuthenticationRequestObject{})
		if err != nil {
			t.Fatalf("BeginPasskeyAuthentication failed: %v", err)
		}
		writeContractResponse(t, optionsRR, optionsResp)
		if optionsRR.Code != http.StatusOK {
			t.Fatalf("expected options status %d, got %d body=%s", http.StatusOK, optionsRR.Code, optionsRR.Body.String())
		}

		finish := func() *httptest.ResponseRecorder {
			req := httptest.NewRequest(
				http.MethodPost,
				"/auth/api/v1/passkeys/authenticate/finish",
				strings.NewReader(`{"challenge_id":`),
			).WithContext(ctx)
			rr := httptest.NewRecorder()
			resp, err := h.FinishPasskeyAuthentication(contractCtx(ctx, req), contract.FinishPasskeyAuthenticationRequestObject{})
			if err != nil {
				t.Fatalf("FinishPasskeyAuthentication failed: %v", err)
			}
			writeContractResponse(t, rr, resp)
			return rr
		}

		if rr := finish(); rr.Code != http.StatusBadRequest {
			t.Fatalf("expected first finish status %d, got %d body=%s", http.StatusBadRequest, rr.Code, rr.Body.String())
		}
		if rr := finish(); rr.Code != http.StatusTooManyRequests || !strings.Contains(rr.Body.String(), "rate_limited") {
			t.Fatalf("expected rate_limited response, got %d body=%s", rr.Code, rr.Body.String())
		}
	})
}

func newAPIPasskeyTestService(t *testing.T, tdb *testutil.TestDB) *passkey.Service {
	t.Helper()

	service, err := passkey.New(passkey.Config{
		RPDisplayName: "Authara",
		RPID:          "localhost",
		RPOrigins:     []string{"http://localhost:3000"},
		Store:         tdb.Store,
		Tx:            tdb.Tx,
		ChallengeTTL:  5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("passkey.New failed: %v", err)
	}
	return service
}
