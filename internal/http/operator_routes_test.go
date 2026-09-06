package http

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/a-h/templ"
	"github.com/authara-org/authara/internal/http/kit/render"
	"github.com/go-chi/chi/v5"
)

const (
	markerOperatorAuthForOperatorRoutes = 471
	markerOperatorRoleForOperatorRoutes = 472
)

func TestOperatorPagesRequireOperatorAuthentication(t *testing.T) {
	router := newOperatorRouteTestRouter(operatorRouteMiddlewareConfig{
		operatorAuth: markerMiddleware(markerOperatorAuthForOperatorRoutes, "operator-auth"),
		operatorRole: passMiddleware,
	})

	for _, path := range []string{"/auth/operator", "/auth/operator/emails", "/auth/operator/emails/signup_code", "/auth/operator/emails/signup_code/versions/1"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != markerOperatorAuthForOperatorRoutes {
			t.Fatalf("GET %s: expected operator auth marker %d, got %d", path, markerOperatorAuthForOperatorRoutes, rr.Code)
		}
	}
}

func TestOperatorPagesRequireExactOperatorRole(t *testing.T) {
	router := newOperatorRouteTestRouter(operatorRouteMiddlewareConfig{
		operatorAuth: passMiddleware,
		operatorRole: markerMiddleware(markerOperatorRoleForOperatorRoutes, "operator-role"),
	})

	for _, path := range []string{"/auth/operator", "/auth/operator/emails", "/auth/operator/emails/signup_code", "/auth/operator/emails/signup_code/versions/1"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != markerOperatorRoleForOperatorRoutes {
			t.Fatalf("GET %s: expected operator role marker %d, got %d", path, markerOperatorRoleForOperatorRoutes, rr.Code)
		}
	}
}

func TestOperatorPagesAreAvailableAfterMiddleware(t *testing.T) {
	router := newOperatorRouteTestRouter(operatorRouteMiddlewareConfig{
		operatorAuth: passMiddleware,
		operatorRole: passMiddleware,
	})

	for _, path := range []string{"/auth/operator", "/auth/operator/", "/auth/operator/emails", "/auth/operator/emails/signup_code"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("GET %s: expected status %d, got %d", path, http.StatusOK, rr.Code)
		}
	}
}

func TestOperatorEmailTemplateMutationsRequireCSRF(t *testing.T) {
	router := newOperatorRouteTestRouter(operatorRouteMiddlewareConfig{
		operatorAuth: passMiddleware,
		operatorRole: passMiddleware,
		csrf:         markerMiddleware(http.StatusTeapot, "csrf"),
	})

	for _, path := range []string{
		"/auth/operator/emails/signup_code",
		"/auth/operator/emails/signup_code/preview",
		"/auth/operator/emails/signup_code/reset",
	} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusTeapot {
			t.Fatalf("POST %s: expected CSRF marker %d, got %d", path, http.StatusTeapot, rr.Code)
		}
	}
}

type operatorRouteMiddlewareConfig struct {
	operatorAuth func(http.Handler) http.Handler
	operatorRole func(http.Handler) http.Handler
	csrf         func(http.Handler) http.Handler
}

func newOperatorRouteTestRouter(m operatorRouteMiddlewareConfig) chi.Router {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if m.operatorAuth == nil {
		m.operatorAuth = passMiddleware
	}
	if m.operatorRole == nil {
		m.operatorRole = passMiddleware
	}
	if m.csrf == nil {
		m.csrf = passMiddleware
	}

	renderer := render.Renderer(func(w http.ResponseWriter, _ *http.Request, status int, _ templ.Component) error {
		w.WriteHeader(status)
		return nil
	})
	cfg := ServerConfig{
		Version:  "test",
		Logger:   logger,
		Handlers: newTestHandlers(logger, renderer),
	}
	mw := Middlewares{
		RedirectIfAuthenticated:              passMiddleware,
		RequireAppAccessAuthWithRefresh:      passMiddleware,
		RequireAppAccessAuthAPI:              passMiddleware,
		RequireAdminAccessAuthWithRefresh:    passMiddleware,
		RequireAdminAccessAuthAPI:            passMiddleware,
		RequireOperatorAccessAuthWithRefresh: m.operatorAuth,
		RequireOperatorAccessAuthAPI:         passMiddleware,
		RequireInternalAPIAuth:               passMiddleware,
		RequirePublicOrganizationManagement:  passMiddleware,
		RequireAdminRole:                     passMiddleware,
		RequireOperatorRole:                  m.operatorRole,
		RequireCSRF:                          m.csrf,
		RequireAPICSRF:                       passMiddleware,
		ReturnTo:                             passMiddleware,
		HTMX:                                 passMiddleware,
		RequireChallengeEnabled:              passMiddleware,
		RequireAllowlistEnabled:              passMiddleware,
		OptionalAppAccessIdentity:            passMiddleware,
	}

	router := chi.NewRouter()
	registerRoutes(router, cfg, mw)
	return router
}
