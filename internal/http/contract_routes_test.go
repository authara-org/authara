package http

import (
	"io"
	"log/slog"
	"net/http"
	"os"
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

	for _, route := range contract.Routes {
		if route.Stability != "stable" {
			continue
		}

		key := route.Method + " " + route.Path
		if !actual[key] {
			t.Fatalf("stable route missing from router: %s", key)
		}
	}
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
