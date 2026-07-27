package http

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/authara-org/authara/internal/http/kit/render"
	httpmiddleware "github.com/authara-org/authara/internal/http/middleware"
	openapicontract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/go-chi/chi/v5"
)

const (
	markerUserAPIAuth  = 419
	markerInternalAuth = 423
)

func TestRouteAccessContract(t *testing.T) {
	document, err := openapicontract.GetSwagger()
	if err != nil {
		t.Fatalf("load generated OpenAPI contract: %v", err)
	}
	router := newAccessContractTestRouter()

	for path, item := range document.Paths.Map() {
		for method, operation := range item.Operations() {
			access, ok := operation.Extensions["x-authara-access"].(string)
			if !ok {
				t.Fatalf("%s is missing x-authara-access", operationKey(method, path))
			}
			t.Run(operationKey(method, path), func(t *testing.T) {
				req := httptest.NewRequest(method, materializeRoutePath(path), nil)
				rr := httptest.NewRecorder()

				router.ServeHTTP(rr, req)

				switch access {
				case "public":
					assertNotAuthMarker(t, rr.Code)

				case "user":
					if rr.Code != markerUserAPIAuth {
						t.Fatalf("expected user API auth marker %d, got %d", markerUserAPIAuth, rr.Code)
					}

				case "internal":
					if rr.Code != markerInternalAuth {
						t.Fatalf("expected internal auth marker %d, got %d", markerInternalAuth, rr.Code)
					}

				default:
					t.Fatalf("unsupported access level %q", access)
				}
			})
		}
	}
}

func newAccessContractTestRouter() chi.Router {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	pass := func(next http.Handler) http.Handler { return next }

	marker := func(status int, body string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				_, _ = w.Write([]byte(body))
			})
		}
	}
	renderer := render.Renderer(func(w http.ResponseWriter, r *http.Request, status int, c templ.Component) error {
		w.WriteHeader(status)
		return nil
	})

	cfg := ServerConfig{
		Version:  "test",
		Addr:     ":0",
		Dev:      true,
		Logger:   logger,
		Handlers: newTestHandlers(logger, renderer),
	}

	mw := Middlewares{
		RedirectIfAuthenticated:   pass,
		ReturnTo:                  pass,
		RequireChallengeEnabled:   pass,
		RequireAllowlistEnabled:   pass,
		HTMX:                      pass,
		RequireCSRF:               pass,
		RequireAPICSRF:            pass,
		OptionalAppAccessIdentity: pass,

		RequireAppAccessAuthWithRefresh:     pass,
		RequireAppAccessAuthAPI:             marker(markerUserAPIAuth, "user-api-auth"),
		RequireAdminAccessAuthWithRefresh:   pass,
		RequireAdminAccessAuthAPI:           pass,
		RequireInternalAPIAuth:              marker(markerInternalAuth, "internal-auth"),
		RequirePublicOrganizationManagement: pass,
		RequireAdminRole:                    pass,
	}

	r := chi.NewRouter()
	r.Use(middlewareRequestLoggerForAccessContract(cfg.Logger))
	registerRoutes(r, cfg, mw)

	return r
}

func materializeRoutePath(path string) string {
	path = strings.ReplaceAll(path, "{userID}", "11111111-1111-1111-1111-111111111111")
	path = strings.ReplaceAll(path, "{organizationID}", "22222222-2222-2222-2222-222222222222")
	path = strings.ReplaceAll(path, "{invitationID}", "33333333-3333-3333-3333-333333333333")
	return path
}

func assertNotAuthMarker(t *testing.T, code int) {
	t.Helper()

	switch code {
	case markerUserAPIAuth, markerInternalAuth:
		t.Fatalf("expected public route, but auth middleware marker intercepted with status %d", code)
	}
}

func middlewareRequestLoggerForAccessContract(logger *slog.Logger) func(http.Handler) http.Handler {
	return httpmiddleware.RequestLogger(logger)
}
