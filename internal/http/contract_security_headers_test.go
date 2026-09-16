package http

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/kit/render"
	"github.com/authara-org/authara/internal/oauth"
)

func TestSecurityHeadersAreAppliedByRouter(t *testing.T) {
	router := newSecurityHeadersTestRouter(oauth.OAuthProviders{})

	req := httptest.NewRequest(http.MethodGet, "/auth/health", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	headers := rr.Result().Header
	assertHeader(t, headers, "Cache-Control", "no-store")
	assertHeader(t, headers, "X-Frame-Options", "DENY")
	assertHeader(t, headers, "X-Content-Type-Options", "nosniff")
	assertHeader(t, headers, "Referrer-Policy", "same-origin")

	csp := headers.Get("Content-Security-Policy")
	for _, expected := range []string{
		"default-src 'self'",
		"frame-ancestors 'none'",
		"object-src 'none'",
		"form-action 'self'",
	} {
		if !strings.Contains(csp, expected) {
			t.Fatalf("expected CSP to contain %q, got %q", expected, csp)
		}
	}
}

func TestSecurityHeadersRouterCSPFollowsGoogleOAuthConfig(t *testing.T) {
	router := newSecurityHeadersTestRouter(oauth.OAuthProviders{
		Providers: []oauth.OAuthProvider{
			{Name: domain.ProviderGoogle, ClientID: "test-client-id"},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/auth/health", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	csp := rr.Result().Header.Get("Content-Security-Policy")
	if !strings.Contains(csp, "https://accounts.google.com") {
		t.Fatalf("expected Google CSP sources when Google OAuth is configured, got %q", csp)
	}
}

func TestDynamicRouterResponsesAreNotCacheable(t *testing.T) {
	router := newSecurityHeadersTestRouter(oauth.OAuthProviders{})
	tests := []struct {
		name        string
		method      string
		target      string
		body        string
		contentType string
		wantStatus  int
	}{
		{
			name:        "HTML success",
			method:      http.MethodGet,
			target:      "/auth/login",
			contentType: "text/html",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "HTML error",
			method:      http.MethodPost,
			target:      "/auth/login",
			contentType: "text/html",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "JSON success",
			method:      http.MethodGet,
			target:      "/auth/version",
			contentType: "application/json",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "JSON validation error",
			method:      http.MethodPost,
			target:      "/auth/api/v1/login",
			body:        "{",
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.target, strings.NewReader(tt.body))
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, rr.Code)
			}
			assertHeader(t, rr.Result().Header, "Cache-Control", "no-store")
			if got := rr.Result().Header.Get("Content-Type"); !strings.HasPrefix(got, tt.contentType) {
				t.Fatalf("expected Content-Type prefix %q, got %q", tt.contentType, got)
			}
		})
	}
}

func newSecurityHeadersTestRouter(providers oauth.OAuthProviders) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pass := func(next http.Handler) http.Handler { return next }
	renderer := render.Renderer(func(w http.ResponseWriter, r *http.Request, status int, c templ.Component) error {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(status)
		return nil
	})

	cfg := ServerConfig{
		Version:        "test",
		Addr:           ":0",
		Dev:            true,
		Logger:         logger,
		OAuthProviders: providers,
		Handlers:       newTestHandlers(logger, renderer),
	}

	mw := Middlewares{
		RedirectIfAuthenticated:              pass,
		RequireAppAccessAuthWithRefresh:      pass,
		RequireAppAccessAuthAPI:              pass,
		RequireAdminAccessAuthWithRefresh:    pass,
		RequireAdminAccessAuthAPI:            pass,
		RequireOperatorAccessAuthWithRefresh: pass,
		RequireOperatorAccessAuthAPI:         pass,
		RequireInternalAPIAuth:               pass,
		RequirePublicOrganizationManagement:  pass,
		RequireAdminRole:                     pass,
		RequireOperatorRole:                  pass,
		RequireCSRF:                          pass,
		RequireAPICSRF:                       pass,
		ReturnTo:                             pass,
		HTMX:                                 pass,
		RequireChallengeEnabled:              pass,
		RequireAllowlistEnabled:              pass,
		OptionalAppAccessIdentity:            pass,
	}

	return NewRouter(cfg, mw)
}

func assertHeader(t *testing.T, headers http.Header, name, expected string) {
	t.Helper()

	if got := headers.Get(name); got != expected {
		t.Fatalf("expected %s %q, got %q", name, expected, got)
	}
}
