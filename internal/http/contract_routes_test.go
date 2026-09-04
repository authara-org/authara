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
	"github.com/authara-org/authara/internal/observability"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-chi/chi/v5"
)

func TestStableRoutesAreRegistered(t *testing.T) {
	document, err := openapicontract.GetSwagger()
	if err != nil {
		t.Fatalf("load generated OpenAPI contract: %v", err)
	}

	router := newContractTestRouter()
	actual := collectRoutes(t, router)
	expected := openAPIRoutes(document)

	for key := range expected {
		if !actual[key] {
			t.Errorf("OpenAPI operation missing from router: %s", key)
		}
	}

	for key := range actual {
		if !isContractAPIRouteKey(key) {
			continue
		}
		if !expected[key] {
			t.Errorf("router operation missing from OpenAPI: %s", key)
		}
	}
}

func TestMetricsRouteIsRegistered(t *testing.T) {
	router := newContractTestRouter()
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	if !strings.Contains(response.Body.String(), `authara_build_info{version="test"} 1`) {
		t.Fatalf("expected build metric, got:\n%s", response.Body.String())
	}
}

func TestMetricsRouteIsNotRegisteredWhenObservabilityIsDisabled(t *testing.T) {
	router := newContractTestRouterWithObservability(nil)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)

	router.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, response.Code)
	}
}

func openAPIRoutes(document *openapi3.T) map[string]bool {
	out := make(map[string]bool)
	for path, item := range document.Paths.Map() {
		for method := range item.Operations() {
			out[operationKey(method, path)] = true
		}
	}
	return out
}

func isContractAPIRouteKey(key string) bool {
	parts := strings.SplitN(key, " ", 2)
	if len(parts) != 2 {
		return false
	}

	path := parts[1]
	return strings.HasPrefix(path, "/auth/api/") || strings.HasPrefix(path, "/auth/internal/")
}

func newContractTestRouter() chi.Router {
	return newContractTestRouterWithObservability(observability.New("test"))
}

func newContractTestRouterWithObservability(metrics *observability.Service) chi.Router {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	pass := func(next http.Handler) http.Handler { return next }
	renderer := render.Renderer(func(w http.ResponseWriter, r *http.Request, status int, c templ.Component) error {
		w.WriteHeader(status)
		return nil
	})

	cfg := ServerConfig{
		Version:                  "test",
		Addr:                     ":0",
		Dev:                      true,
		Logger:                   logger,
		Observability:            metrics,
		Handlers:                 newTestHandlers(logger, renderer),
		disableOpenAPIValidation: true,
	}

	mw := Middlewares{
		RedirectIfAuthenticated:             pass,
		RequireAppAccessAuthWithRefresh:     pass,
		RequireAppAccessAuthAPI:             pass,
		RequireAdminAccessAuthWithRefresh:   pass,
		RequireAdminAccessAuthAPI:           pass,
		RequireInternalAPIAuth:              pass,
		RequirePublicOrganizationManagement: pass,
		RequireAdminRole:                    pass,
		RequireCSRF:                         pass,
		RequireAPICSRF:                      pass,
		ReturnTo:                            pass,
		HTMX:                                pass,
		RequireChallengeEnabled:             pass,
		RequireAllowlistEnabled:             pass,
		OptionalAppAccessIdentity:           pass,
	}

	r := chi.NewRouter()

	if metrics != nil {
		r.Use(metrics.Middleware)
	}
	r.Use(middlewareRequestLogger(cfg.Logger))

	registerRoutes(r, cfg, mw)

	return r
}

func collectRoutes(t *testing.T, r chi.Router) map[string]bool {
	t.Helper()

	out := make(map[string]bool)

	err := chi.Walk(r, func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		out[method+" "+route] = true
		return nil
	})
	if err != nil {
		t.Fatalf("walk routes: %v", err)
	}

	return out
}

func middlewareRequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return httpmiddleware.RequestLogger(logger)
}
