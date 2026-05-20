package http

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/a-h/templ"
	adminsvc "github.com/authara-org/authara/internal/admin"
	"github.com/authara-org/authara/internal/features"
	"github.com/authara-org/authara/internal/http/kit/render"
	httpmiddleware "github.com/authara-org/authara/internal/http/middleware"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/go-chi/chi/v5"
)

const (
	markerAdminAuthForAdminRoutes = 461
	markerAdminRoleForAdminRoutes = 462
	markerAdminCSRFForAdminRoutes = 463
)

func TestAdminPagesRequireAdminAuthentication(t *testing.T) {
	router := newAdminRouteTestRouter(adminRouteMiddlewareConfig{
		adminAuth: markerMiddleware(markerAdminAuthForAdminRoutes, "admin-auth"),
		adminRole: passMiddleware,
		csrf:      passMiddleware,
		allowlist: httpmiddleware.RequireAllowlistEnabled(true),
	})

	req := httptest.NewRequest(http.MethodGet, "/auth/admin/users", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != markerAdminAuthForAdminRoutes {
		t.Fatalf("expected admin auth marker %d, got %d", markerAdminAuthForAdminRoutes, rr.Code)
	}
}

func TestAdminAllowlistResultsRequireAdminAuthentication(t *testing.T) {
	router := newAdminRouteTestRouter(adminRouteMiddlewareConfig{
		adminAuth: markerMiddleware(markerAdminAuthForAdminRoutes, "admin-auth"),
		adminRole: passMiddleware,
		csrf:      passMiddleware,
		allowlist: httpmiddleware.RequireAllowlistEnabled(true),
	})

	req := httptest.NewRequest(http.MethodGet, "/auth/admin/allowlist/results", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != markerAdminAuthForAdminRoutes {
		t.Fatalf("expected admin auth marker %d, got %d", markerAdminAuthForAdminRoutes, rr.Code)
	}
}

func TestAdminPagesRequireAdminRole(t *testing.T) {
	router := newAdminRouteTestRouter(adminRouteMiddlewareConfig{
		adminAuth: passMiddleware,
		adminRole: markerMiddleware(markerAdminRoleForAdminRoutes, "admin-role"),
		csrf:      passMiddleware,
		allowlist: httpmiddleware.RequireAllowlistEnabled(true),
	})

	req := httptest.NewRequest(http.MethodGet, "/auth/admin", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != markerAdminRoleForAdminRoutes {
		t.Fatalf("expected admin role marker %d, got %d", markerAdminRoleForAdminRoutes, rr.Code)
	}
}

func TestAdminAllowlistResultsRequireAdminRole(t *testing.T) {
	router := newAdminRouteTestRouter(adminRouteMiddlewareConfig{
		adminAuth: passMiddleware,
		adminRole: markerMiddleware(markerAdminRoleForAdminRoutes, "admin-role"),
		csrf:      passMiddleware,
		allowlist: httpmiddleware.RequireAllowlistEnabled(true),
	})

	req := httptest.NewRequest(http.MethodGet, "/auth/admin/allowlist/results", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != markerAdminRoleForAdminRoutes {
		t.Fatalf("expected admin role marker %d, got %d", markerAdminRoleForAdminRoutes, rr.Code)
	}
}

func TestAdminMutationsRequireCSRF(t *testing.T) {
	router := newAdminRouteTestRouter(adminRouteMiddlewareConfig{
		adminAuth: passMiddleware,
		adminRole: passMiddleware,
		csrf:      markerMiddleware(markerAdminCSRFForAdminRoutes, "admin-csrf"),
	})

	req := httptest.NewRequest(http.MethodPost, "/auth/admin/users/11111111-1111-1111-1111-111111111111/disable", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != markerAdminCSRFForAdminRoutes {
		t.Fatalf("expected admin CSRF marker %d, got %d", markerAdminCSRFForAdminRoutes, rr.Code)
	}
}

func TestAdminAllowlistRoutesReturnNotFoundWhenDisabled(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		router := newAdminRouteTestRouterWithConfig(adminRouteMiddlewareConfig{
			adminAuth: passMiddleware,
			adminRole: passMiddleware,
			csrf:      markerMiddleware(markerAdminCSRFForAdminRoutes, "admin-csrf"),
			allowlist: httpmiddleware.RequireAllowlistEnabled(false),
		}, ServerConfig{
			Admin: adminsvc.New(adminsvc.Config{
				Store: tdb.Store,
				Tx:    tdb.Tx,
			}),
			Features: features.Features{AllowlistEnabled: false},
		})

		for _, tc := range []struct {
			method string
			path   string
		}{
			{method: http.MethodGet, path: "/auth/admin/allowlist"},
			{method: http.MethodGet, path: "/auth/admin/allowlist/results"},
			{method: http.MethodPost, path: "/auth/admin/allowlist"},
			{method: http.MethodPost, path: "/auth/admin/allowlist/11111111-1111-1111-1111-111111111111/delete"},
		} {
			req := httptest.NewRequest(tc.method, tc.path, nil).WithContext(ctx)
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			if rr.Code != http.StatusNotFound {
				t.Fatalf("%s %s: expected status %d, got %d body=%s", tc.method, tc.path, http.StatusNotFound, rr.Code, rr.Body.String())
			}
		}
	})
}

func TestAdminAllowlistPageAvailableWhenEnabled(t *testing.T) {
	router := newAdminRouteTestRouterWithConfig(adminRouteMiddlewareConfig{
		adminAuth: passMiddleware,
		adminRole: passMiddleware,
		csrf:      passMiddleware,
		allowlist: httpmiddleware.RequireAllowlistEnabled(true),
	}, ServerConfig{
		Features: features.Features{AllowlistEnabled: true},
	})

	req := httptest.NewRequest(http.MethodGet, "/auth/admin/allowlist", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestAdminAllowlistMutationsRequireCSRFWhenEnabled(t *testing.T) {
	router := newAdminRouteTestRouterWithConfig(adminRouteMiddlewareConfig{
		adminAuth: passMiddleware,
		adminRole: passMiddleware,
		csrf:      markerMiddleware(markerAdminCSRFForAdminRoutes, "admin-csrf"),
		allowlist: httpmiddleware.RequireAllowlistEnabled(true),
	}, ServerConfig{
		Features: features.Features{AllowlistEnabled: true},
	})

	for _, path := range []string{
		"/auth/admin/allowlist",
		"/auth/admin/allowlist/11111111-1111-1111-1111-111111111111/delete",
	} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != markerAdminCSRFForAdminRoutes {
			t.Fatalf("POST %s: expected admin CSRF marker %d, got %d", path, markerAdminCSRFForAdminRoutes, rr.Code)
		}
	}
}

func TestAdminAllowlistResultsDoesNotRequireCSRF(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		router := newAdminRouteTestRouterWithConfig(adminRouteMiddlewareConfig{
			adminAuth: passMiddleware,
			adminRole: passMiddleware,
			csrf:      markerMiddleware(markerAdminCSRFForAdminRoutes, "admin-csrf"),
			allowlist: httpmiddleware.RequireAllowlistEnabled(true),
		}, ServerConfig{
			Admin: adminsvc.New(adminsvc.Config{
				Store:            tdb.Store,
				Tx:               tdb.Tx,
				AllowlistEnabled: true,
			}),
			Features: features.Features{AllowlistEnabled: true},
		})

		req := httptest.NewRequest(http.MethodGet, "/auth/admin/allowlist/results", nil).WithContext(ctx)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code == markerAdminCSRFForAdminRoutes {
			t.Fatalf("GET allowlist results should not require CSRF")
		}
		if rr.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rr.Code, rr.Body.String())
		}
	})
}

type adminRouteMiddlewareConfig struct {
	adminAuth func(http.Handler) http.Handler
	adminRole func(http.Handler) http.Handler
	csrf      func(http.Handler) http.Handler
	allowlist func(http.Handler) http.Handler
}

func newAdminRouteTestRouter(m adminRouteMiddlewareConfig) chi.Router {
	return newAdminRouteTestRouterWithConfig(m, ServerConfig{})
}

func newAdminRouteTestRouterWithConfig(m adminRouteMiddlewareConfig, override ServerConfig) chi.Router {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if m.adminAuth == nil {
		m.adminAuth = passMiddleware
	}
	if m.adminRole == nil {
		m.adminRole = passMiddleware
	}
	if m.csrf == nil {
		m.csrf = passMiddleware
	}
	if m.allowlist == nil {
		m.allowlist = passMiddleware
	}
	cfg := ServerConfig{
		Version: "test",
		Logger:  logger,
		Render: render.Renderer(func(w http.ResponseWriter, r *http.Request, status int, c templ.Component) error {
			w.WriteHeader(status)
			return nil
		}),
	}
	if override.Admin != nil {
		cfg.Admin = override.Admin
	}
	cfg.Features = override.Features
	mw := Middlewares{
		RedirectIfAuthenticated:           passMiddleware,
		RequireAppAccessAuthWithRefresh:   passMiddleware,
		RequireAppAccessAuthAPI:           passMiddleware,
		RequireAdminAccessAuthWithRefresh: m.adminAuth,
		RequireAdminAccessAuthAPI:         m.adminAuth,
		RequireAdminRole:                  m.adminRole,
		RequireCSRF:                       m.csrf,
		RequireAPICSRF:                    m.csrf,
		ReturnTo:                          passMiddleware,
		HTMX:                              passMiddleware,
		RequireChallengeEnabled:           passMiddleware,
		RequireAllowlistEnabled:           m.allowlist,
		OptionalAppAccessIdentity:         passMiddleware,
	}

	r := chi.NewRouter()
	registerRoutes(r, cfg, mw)
	return r
}

func passMiddleware(next http.Handler) http.Handler {
	return next
}

func markerMiddleware(status int, body string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, body, status)
		})
	}
}
