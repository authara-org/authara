package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/authara-org/authara/internal/http/kit/csrf"
	"github.com/authara-org/authara/internal/http/kit/httpctx"
	"github.com/authara-org/authara/internal/session"
)

func TestAPIAccessTokenPrefersBearerAndRejectsMalformedAuthorization(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Bearer native-token")
	addAccessCookie(request, "cookie-token")

	tokenValue, transport, ok := apiAccessToken(request)
	if !ok || tokenValue != "native-token" || transport != httpctx.AuthTransportBearer {
		t.Fatalf("token = %q, transport = %q, ok = %v", tokenValue, transport, ok)
	}

	request.Header.Set("Authorization", "Basic malformed")
	if tokenValue, _, ok := apiAccessToken(request); ok || tokenValue != "" {
		t.Fatalf("malformed Authorization fell back to cookie: token = %q, ok = %v", tokenValue, ok)
	}

	request.Header.Set("Authorization", "   ")
	if tokenValue, _, ok := apiAccessToken(request); ok || tokenValue != "" {
		t.Fatalf("blank Authorization fell back to cookie: token = %q, ok = %v", tokenValue, ok)
	}
}

func TestAPIAccessTokenUsesCookieWithoutAuthorization(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()
	session.SetAccessToken(recorder, "cookie-token", 60)
	request.AddCookie(recorder.Result().Cookies()[0])

	tokenValue, transport, ok := apiAccessToken(request)
	if !ok || tokenValue != "cookie-token" || transport != httpctx.AuthTransportCookie {
		t.Fatalf("token = %q, transport = %q, ok = %v", tokenValue, transport, ok)
	}
}

func TestRequireAPICSRFBypassesOnlyAuthenticatedBearerTransport(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	handler := RequireAPICSRF(next)

	bearer := httptest.NewRequest(http.MethodPost, "/", nil)
	bearer = bearer.WithContext(httpctx.WithAuthTransport(bearer.Context(), httpctx.AuthTransportBearer))
	bearerRecorder := httptest.NewRecorder()
	handler.ServeHTTP(bearerRecorder, bearer)
	if bearerRecorder.Code != http.StatusNoContent {
		t.Fatalf("bearer status = %d", bearerRecorder.Code)
	}

	cookie := httptest.NewRequest(http.MethodPost, "/", nil)
	cookie = cookie.WithContext(httpctx.WithAuthTransport(cookie.Context(), httpctx.AuthTransportCookie))
	cookieRecorder := httptest.NewRecorder()
	handler.ServeHTTP(cookieRecorder, cookie)
	if cookieRecorder.Code != http.StatusForbidden {
		t.Fatalf("cookie status = %d, want 403", cookieRecorder.Code)
	}
}

func TestRequirePublicAPICSRFBypassesOnlyTokenSessionTransport(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	handler := RequirePublicAPICSRF(next)

	for _, tc := range []struct {
		url  string
		want int
	}{
		{url: "/?session_transport=token", want: http.StatusNoContent},
		{url: "/?session_transport=cookie", want: http.StatusForbidden},
		{url: "/?session_transport=TOKEN", want: http.StatusForbidden},
		{url: "/", want: http.StatusForbidden},
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, tc.url, nil))
		if recorder.Code != tc.want {
			t.Errorf("%s status = %d, want %d", tc.url, recorder.Code, tc.want)
		}
	}

	validBrowser := httptest.NewRequest(http.MethodPost, "/", nil)
	validBrowser.AddCookie(&http.Cookie{Name: csrf.CookieName, Value: "csrf-token"})
	validBrowser.Header.Set("X-CSRF-Token", "csrf-token")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, validBrowser)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("valid browser status = %d", recorder.Code)
	}
}
