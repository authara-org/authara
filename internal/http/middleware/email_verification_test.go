package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/session/token"
	"github.com/google/uuid"
)

type fakeEmailVerificationStarter struct {
	transaction domain.EmailVerificationTransaction
	called      bool
	userID      uuid.UUID
	sessionID   uuid.UUID
	audience    string
	returnTo    string
}

func (f *fakeEmailVerificationStarter) BeginRequiredEmailVerification(
	_ context.Context,
	userID uuid.UUID,
	sessionID uuid.UUID,
	audience string,
	returnTo string,
	_ time.Time,
) (domain.EmailVerificationTransaction, error) {
	f.called = true
	f.userID = userID
	f.sessionID = sessionID
	f.audience = audience
	f.returnTo = returnTo
	return f.transaction, nil
}

func TestRequireVerifiedEmailUIRevokesNormalAccessAndRedirects(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	userID, sessionID, transactionID := uuid.New(), uuid.New(), uuid.New()
	starter := &fakeEmailVerificationStarter{transaction: domain.EmailVerificationTransaction{
		ID: transactionID, ExpiresAt: now.Add(30 * time.Minute),
	}}
	policy := config.AuthenticationPolicyReaderFunc(func() config.AuthenticationPolicy {
		return config.AuthenticationPolicy{EmailVerificationRequired: true}
	})
	nextCalled := false
	handler := RequireVerifiedEmailUI(starter, policy, token.AudienceApp, func() time.Time { return now })(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		nextCalled = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/auth/account?tab=security", nil)
	req.AddCookie(&http.Cookie{Name: "authara_access", Value: "access"})
	req.AddCookie(&http.Cookie{Name: "authara_refresh", Value: "refresh"})
	ctx := httpctx.WithUserID(req.Context(), userID)
	ctx = httpctx.WithSessionID(ctx, sessionID)
	ctx = httpctx.WithEmailVerified(ctx, false)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if nextCalled {
		t.Fatal("protected handler was called")
	}
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/auth/verify-email" {
		t.Fatalf("response = %d %q", rr.Code, rr.Header().Get("Location"))
	}
	if !starter.called || starter.userID != userID || starter.sessionID != sessionID || starter.audience != "app" || starter.returnTo != "/auth/account?tab=security" {
		t.Fatalf("unexpected verification handoff: %+v", starter)
	}
	verificationRequest := httptest.NewRequest(http.MethodGet, "/auth/verify-email", nil)
	for _, cookie := range rr.Result().Cookies() {
		verificationRequest.AddCookie(cookie)
	}
	gotID, ok := session.ReadEmailVerificationTransaction(verificationRequest)
	if !ok || gotID != transactionID {
		t.Fatalf("verification cookie = %v, %t", gotID, ok)
	}
}

func TestRequireVerifiedEmailAllowsVerifiedUser(t *testing.T) {
	starter := &fakeEmailVerificationStarter{}
	policy := config.AuthenticationPolicyReaderFunc(func() config.AuthenticationPolicy {
		return config.AuthenticationPolicy{EmailVerificationRequired: true}
	})
	nextCalled := false
	handler := RequireVerifiedEmailAPI(starter, policy, token.AudienceApp, time.Now)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		nextCalled = true
	}))
	req := httptest.NewRequest(http.MethodGet, "/auth/api/v1/user", nil)
	req = req.WithContext(httpctx.WithEmailVerified(req.Context(), true))

	handler.ServeHTTP(httptest.NewRecorder(), req)

	if !nextCalled || starter.called {
		t.Fatal("verified request was not passed through")
	}
}

func TestRequireVerifiedEmailAllowsStaleClaimAfterConcurrentVerification(t *testing.T) {
	starter := &fakeEmailVerificationStarter{}
	policy := config.AuthenticationPolicyReaderFunc(func() config.AuthenticationPolicy {
		return config.AuthenticationPolicy{EmailVerificationRequired: true}
	})
	nextCalled := false
	handler := RequireVerifiedEmailAPI(starter, policy, token.AudienceApp, time.Now)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		nextCalled = true
	}))
	req := httptest.NewRequest(http.MethodGet, "/auth/api/v1/user", nil)
	ctx := httpctx.WithUserID(req.Context(), uuid.New())
	ctx = httpctx.WithSessionID(ctx, uuid.New())
	ctx = httpctx.WithEmailVerified(ctx, false)
	req = req.WithContext(ctx)

	handler.ServeHTTP(httptest.NewRecorder(), req)

	if !starter.called || !nextCalled {
		t.Fatal("request with a concurrently verified database user was not passed through")
	}
}
