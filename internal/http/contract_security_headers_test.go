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
	"github.com/authara-org/authara/internal/ratelimiter"
)

func TestSecurityHeadersAreAppliedByRouter(t *testing.T) {
	router := newSecurityHeadersTestRouter(oauth.OAuthProviders{})

	req := httptest.NewRequest(http.MethodGet, "/auth/health", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	headers := rr.Result().Header
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

func newSecurityHeadersTestRouter(providers oauth.OAuthProviders) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pass := func(next http.Handler) http.Handler { return next }

	cfg := ServerConfig{
		Version:        "test",
		Addr:           ":0",
		Dev:            true,
		Logger:         logger,
		AuthLimiter:    ratelimiter.AuthLimiter(nil),
		OAuthProviders: providers,
		Render: render.Renderer(func(w http.ResponseWriter, r *http.Request, status int, c templ.Component) error {
			w.WriteHeader(status)
			return nil
		}),
	}

	mw := Middlewares{
		RedirectIfAuthenticated:           pass,
		RequireAppAccessAuthWithRefresh:   pass,
		RequireAppAccessAuthAPI:           pass,
		RequireAdminAccessAuthWithRefresh: pass,
		RequireAdminAccessAuthAPI:         pass,
		RequireAdminRole:                  pass,
		RequireCSRF:                       pass,
		RequireAPICSRF:                    pass,
		ReturnTo:                          pass,
		HTMX:                              pass,
		RequireChallengeEnabled:           pass,
		OptionalAppAccessIdentity:         pass,
	}

	return NewRouter(cfg, mw)
}

func assertHeader(t *testing.T, headers http.Header, name, expected string) {
	t.Helper()

	if got := headers.Get(name); got != expected {
		t.Fatalf("expected %s %q, got %q", name, expected, got)
	}
}
