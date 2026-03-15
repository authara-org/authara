package http

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/a-h/templ"
	"github.com/authara-org/authara/internal/http/kit/render"
	httpmiddleware "github.com/authara-org/authara/internal/http/middleware"
	"github.com/authara-org/authara/internal/oauth/google"
	"github.com/authara-org/authara/internal/ratelimiter"
	"github.com/go-chi/chi/v5"
	"gopkg.in/yaml.v3"
)

type httpContractRoutes struct {
	Version int             `yaml:"version"`
	Routes  []contractRoute `yaml:"routes"`
}

type contractRoute struct {
	Method    string `yaml:"method"`
	Path      string `yaml:"path"`
	Stability string `yaml:"stability"`
	Kind      string `yaml:"kind"`
	Access    string `yaml:"access"`
}

func TestStableRoutesAreRegistered(t *testing.T) {
	data, err := os.ReadFile("../../contract/http.yaml")
	if err != nil {
		t.Fatalf("read contract/http.yaml: %v", err)
	}

	var contract httpContractRoutes
	if err := yaml.Unmarshal(data, &contract); err != nil {
		t.Fatalf("unmarshal contract/http.yaml: %v", err)
	}

	router := newContractTestRouter()
	actual := collectRoutes(t, router)
	expected := stableContractRoutes(contract)

	// Contract -> router for all stable contract routes.
	for key := range expected {
		if !actual[key] {
			t.Fatalf("stable route missing from router: %s", key)
		}
	}

	// Router -> contract only for public API routes.
	for key := range actual {
		if !isPublicAPIRouteKey(key) {
			continue
		}
		if !expected[key] {
			t.Fatalf("public API route missing from stable contract: %s", key)
		}
	}
}

func stableContractRoutes(contract httpContractRoutes) map[string]bool {
	out := make(map[string]bool)

	for _, route := range contract.Routes {
		if route.Stability != "stable" {
			continue
		}
		out[route.Method+" "+route.Path] = true
	}

	return out
}

func isPublicAPIRouteKey(key string) bool {
	parts := strings.SplitN(key, " ", 2)
	if len(parts) != 2 {
		return false
	}

	path := parts[1]
	return strings.HasPrefix(path, "/auth/api")
}

func newContractTestRouter() chi.Router {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	pass := func(next http.Handler) http.Handler { return next }

	cfg := ServerConfig{
		Version:         "test",
		Addr:            ":0",
		Auth:            nil,
		Dev:             true,
		Session:         nil,
		Logger:          logger,
		Store:           nil,
		AuthLimiter:     ratelimiter.AuthLimiter(nil),
		Google:          google.New("test-client-id"),
		AccessTokenTTL:  10 * time.Minute,
		RefreshTokenTTL: 24 * time.Hour,
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
	}

	r := chi.NewRouter()

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
