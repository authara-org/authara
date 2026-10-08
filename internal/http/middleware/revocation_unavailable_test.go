package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/cache"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/session/roles"
	"github.com/authara-org/authara/internal/session/token"
	"github.com/google/uuid"
)

func TestRequireAPIAccessAuthReturnsServiceUnavailableWhenRevocationCheckFails(t *testing.T) {
	sessionService, accessToken, now := newRevocationUnavailableSessionService(t)
	called := false
	handler := RequireAPIAccessAuth(sessionService, token.AudienceApp, func() time.Time { return now })(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }),
	)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	addAccessCookie(request, accessToken)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable || called {
		t.Fatalf("status = %d, next called = %v", recorder.Code, called)
	}
}

func TestRequirePageAccessAuthDoesNotRefreshOrClearCookiesWhenRevocationCheckFails(t *testing.T) {
	sessionService, accessToken, now := newRevocationUnavailableSessionService(t)
	called := false
	handler := RequireAccessAuthWithRefresh(
		sessionService,
		token.AudienceApp,
		10*time.Minute,
		time.Hour,
		func() time.Time { return now },
	)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	addAccessCookie(request, accessToken)
	refreshRecorder := httptest.NewRecorder()
	session.SetRefreshToken(refreshRecorder, "refresh", 3600)
	request.AddCookie(refreshRecorder.Result().Cookies()[0])

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable || called {
		t.Fatalf("status = %d, next called = %v", recorder.Code, called)
	}
	if got := recorder.Header().Values("Set-Cookie"); len(got) != 0 {
		t.Fatalf("Set-Cookie = %v, want existing cookies left untouched", got)
	}
}

func newRevocationUnavailableSessionService(t *testing.T) (*session.Service, string, time.Time) {
	t.Helper()
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	keySet, err := token.NewKeySet("test", map[string][]byte{
		"test": []byte("01234567890123456789012345678901"),
	})
	if err != nil {
		t.Fatal(err)
	}
	accessTokens := token.NewAccessTokenService(keySet, "authara-test", 10*time.Minute)
	accessToken, err := accessTokens.Generate(
		uuid.New(), uuid.New(), uuid.New(), "member", token.AudienceApp, roles.Roles{}, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	return session.New(session.SessionConfig{
		AccessTokens: accessTokens,
		AccessTokenRevocations: token.NewAccessTokenRevocations(
			unavailableRevocationCache{},
			time.Hour,
		),
	}), accessToken, now
}

func addAccessCookie(request *http.Request, accessToken string) {
	recorder := httptest.NewRecorder()
	session.SetAccessToken(recorder, accessToken, 600)
	request.AddCookie(recorder.Result().Cookies()[0])
}

type unavailableRevocationCache struct{}

func (unavailableRevocationCache) Get(context.Context, string) ([]byte, error) {
	return nil, cache.ErrMiss
}

func (unavailableRevocationCache) GetMany(context.Context, ...string) ([][]byte, error) {
	return nil, errors.New("redis unavailable")
}

func (unavailableRevocationCache) Set(context.Context, string, []byte, time.Duration) error {
	return errors.New("redis unavailable")
}

func (unavailableRevocationCache) SetMaxInt64(context.Context, string, int64, time.Duration) error {
	return errors.New("redis unavailable")
}

func (unavailableRevocationCache) Delete(context.Context, string) error { return nil }
func (unavailableRevocationCache) Close() error                         { return nil }
