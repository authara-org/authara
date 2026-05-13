package ui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	authsvc "github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/http/kit/render"
	"github.com/authara-org/authara/internal/oauth/google"
	"github.com/authara-org/authara/internal/passkey"
	"github.com/authara-org/authara/internal/ratelimiter"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestPasskeyRegisterOptions_UnauthenticatedRejected(t *testing.T) {
	h := &UIHandler{
		Passkeys: nil,
		Render:   render.New(render.Assets{}, false),
	}

	req := httptest.NewRequest(http.MethodPost, "/auth/passkeys/register/options", nil)
	rr := httptest.NewRecorder()

	h.PasskeyRegisterOptionsPost(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestPasskeyRegisterOptions_AuthenticatedReturnsOptions(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user := createUIPasskeyUser(t, ctx, tdb, "options-passkey@example.com", "options-passkey")
		h := newUIPasskeyTestHandler(t, tdb)

		req := httptest.NewRequest(http.MethodPost, "/auth/passkeys/register/options", nil)
		req = req.WithContext(httpctx.WithUserID(ctx, user.ID))
		rr := httptest.NewRecorder()

		h.PasskeyRegisterOptionsPost(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rr.Code, rr.Body.String())
		}

		var body struct {
			ChallengeID string `json:"challenge_id"`
			Options     struct {
				PublicKey struct {
					Challenge string `json:"challenge"`
					User      struct {
						ID string `json:"id"`
					} `json:"user"`
					AuthenticatorSelection struct {
						ResidentKey      string `json:"residentKey"`
						UserVerification string `json:"userVerification"`
					} `json:"authenticatorSelection"`
				} `json:"publicKey"`
			} `json:"options"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if body.ChallengeID == "" || body.Options.PublicKey.Challenge == "" || body.Options.PublicKey.User.ID == "" {
			t.Fatalf("expected challenge id, challenge, and user id in response: %+v", body)
		}
		if body.Options.PublicKey.AuthenticatorSelection.ResidentKey != "required" {
			t.Fatalf("expected resident key required, got %q", body.Options.PublicKey.AuthenticatorSelection.ResidentKey)
		}
		if body.Options.PublicKey.AuthenticatorSelection.UserVerification != "required" {
			t.Fatalf("expected user verification required, got %q", body.Options.PublicKey.AuthenticatorSelection.UserVerification)
		}
	})
}

func TestPasskeyAuthenticateOptionsRequiresUserVerification(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		h := newUIPasskeyTestHandler(t, tdb)

		req := httptest.NewRequest(http.MethodPost, "/auth/passkeys/authenticate/options", nil)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		h.PasskeyAuthenticateOptionsPost(rr, req)

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
			t.Fatalf("unmarshal response: %v", err)
		}
		if body.ChallengeID == "" || body.Options.PublicKey.Challenge == "" {
			t.Fatalf("expected challenge id and challenge in response: %+v", body)
		}
		if body.Options.PublicKey.UserVerification != "required" {
			t.Fatalf("expected user verification required, got %q", body.Options.PublicKey.UserVerification)
		}
	})
}

func TestPasskeyAuthenticateOptionsRateLimited(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		h := newUIPasskeyTestHandler(t, tdb)
		h.Limiter = ratelimiter.NewInMemoryLimiter(ratelimiter.LimiterConfig{
			PasskeyLoginIPLimit:  1,
			PasskeyLoginIPWindow: time.Hour,
		})

		req := httptest.NewRequest(http.MethodPost, "/auth/passkeys/authenticate/options", nil)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		h.PasskeyAuthenticateOptionsPost(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected first request status %d, got %d body=%s", http.StatusOK, rr.Code, rr.Body.String())
		}

		req = httptest.NewRequest(http.MethodPost, "/auth/passkeys/authenticate/options", nil)
		req = req.WithContext(ctx)
		rr = httptest.NewRecorder()

		h.PasskeyAuthenticateOptionsPost(rr, req)
		if rr.Code != http.StatusTooManyRequests {
			t.Fatalf("expected second request status %d, got %d body=%s", http.StatusTooManyRequests, rr.Code, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "rate_limited") {
			t.Fatalf("expected rate_limited error body, got %s", rr.Body.String())
		}
	})
}

func TestLoginPageIncludesPasskeyControls(t *testing.T) {
	h := &UIHandler{
		Render: render.New(render.Assets{}, false),
	}

	req := httptest.NewRequest(http.MethodGet, "/auth/login?return_to=%2Fdashboard", nil)
	req = req.WithContext(httpctx.WithReturnTo(req.Context(), "/dashboard"))
	rr := httptest.NewRecorder()

	h.LoginPage(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "data-passkey-login") {
		t.Fatal("expected login page to include passkey button")
	}
	if !strings.Contains(body, `autocomplete="username webauthn"`) {
		t.Fatal("expected login email input to enable passkey autofill")
	}
	if !strings.Contains(body, `data-passkey-conditional-login="true"`) {
		t.Fatal("expected login page to include conditional passkey marker")
	}
	if !strings.Contains(body, "data-login-form") {
		t.Fatal("expected login form to include password-submit abort marker")
	}
	if !strings.Contains(body, `data-return-to="/dashboard"`) {
		t.Fatal("expected conditional passkey marker to include normalized return_to")
	}
}

func TestAccountPageIncludesPasskeysSubsection(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user := createUIPasswordUser(t, ctx, tdb, "account-passkeys@example.com", "account-passkeys")
		h := newUIPasskeyTestHandler(t, tdb)

		req := httptest.NewRequest(http.MethodGet, "/auth/account", nil)
		req = req.WithContext(httpctx.WithSessionID(httpctx.WithUserID(ctx, user.ID), uuid.New()))
		rr := httptest.NewRecorder()

		h.AccountGet(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rr.Code, rr.Body.String())
		}
		body := rr.Body.String()
		if !strings.Contains(body, "Passkeys") || !strings.Contains(body, "No passkeys added.") {
			t.Fatalf("expected account page passkeys subsection, body=%s", body)
		}
	})
}

func TestPasskeyDeleteLastMethodReturns422Toast(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user := createUIPasskeyUser(t, ctx, tdb, "delete-last-passkey@example.com", "delete-last-passkey")
		p := createUIPasskey(t, ctx, tdb, user.ID, "delete-last-passkey-credential")
		h := newUIPasskeyTestHandler(t, tdb)

		req := httptest.NewRequest(http.MethodPost, "/auth/passkeys/"+p.ID.String()+"/delete", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", p.ID.String())
		req = req.WithContext(context.WithValue(httpctx.WithUserID(ctx, user.ID), chi.RouteCtxKey, rctx))
		rr := httptest.NewRecorder()

		h.PasskeyDeletePost(rr, req)

		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected status %d, got %d body=%s", http.StatusUnprocessableEntity, rr.Code, rr.Body.String())
		}
		if !strings.Contains(rr.Body.String(), "You need at least one sign-in method.") {
			t.Fatalf("expected last-method toast, body=%s", rr.Body.String())
		}
	})
}

func TestPasskeyNormalizedReturnToRejectsUnsafeValues(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		fallback string
		want     string
	}{
		{
			name:     "relative path",
			raw:      "/dashboard",
			fallback: "/auth/account",
			want:     "/dashboard",
		},
		{
			name:     "protocol relative ignored",
			raw:      "//evil.com",
			fallback: "/auth/account",
			want:     "/auth/account",
		},
		{
			name:     "absolute url ignored",
			raw:      "https://evil.com",
			fallback: "/auth/account",
			want:     "/auth/account",
		},
		{
			name:     "unsafe fallback ignored",
			raw:      "//evil.com",
			fallback: "//also-evil.com",
			want:     "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizedReturnTo(tt.raw, tt.fallback)
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func newUIPasskeyTestHandler(t *testing.T, tdb *testutil.TestDB) *UIHandler {
	t.Helper()

	passkeys, err := passkey.New(passkey.Config{
		RPDisplayName: "Authara",
		RPID:          "localhost",
		RPOrigins:     []string{"http://localhost:3000"},
		Store:         tdb.Store,
		Tx:            tdb.Tx,
	})
	if err != nil {
		t.Fatalf("passkey.New failed: %v", err)
	}

	return &UIHandler{
		Auth: authsvc.New(authsvc.Config{
			Store: tdb.Store,
			Tx:    tdb.Tx,
		}),
		Passkeys: passkeys,
		Session: session.New(session.SessionConfig{
			Store:           tdb.Store,
			Tx:              tdb.Tx,
			SessionTTL:      time.Hour,
			RefreshTokenTTL: 24 * time.Hour,
		}),
		Google:     google.New("test-client-id"),
		AccessTTL:  time.Minute,
		RefreshTTL: 24 * time.Hour,
		Render:     render.New(render.Assets{}, false),
	}
}

func createUIPasskeyUser(
	t *testing.T,
	ctx context.Context,
	tdb *testutil.TestDB,
	email string,
	username string,
) domain.User {
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

func createUIPasswordUser(
	t *testing.T,
	ctx context.Context,
	tdb *testutil.TestDB,
	email string,
	username string,
) domain.User {
	t.Helper()

	user := createUIPasskeyUser(t, ctx, tdb, email, username)
	hash := "hashed-password"
	_, err := tdb.Store.CreateAuthProvider(ctx, domain.AuthProvider{
		UserID:       user.ID,
		Provider:     domain.ProviderPassword,
		PasswordHash: &hash,
	})
	if err != nil {
		t.Fatalf("CreateAuthProvider failed: %v", err)
	}
	return user
}

func createUIPasskey(
	t *testing.T,
	ctx context.Context,
	tdb *testutil.TestDB,
	userID uuid.UUID,
	credentialID string,
) domain.Passkey {
	t.Helper()

	p, err := tdb.Store.CreatePasskey(ctx, domain.Passkey{
		UserID:       userID,
		CredentialID: []byte(credentialID),
		PublicKey:    []byte("public-key-" + credentialID),
		Name:         "Passkey",
	})
	if err != nil {
		t.Fatalf("CreatePasskey failed: %v", err)
	}
	return p
}
