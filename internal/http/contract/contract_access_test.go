package contract

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
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

type accessContract struct {
	Routes []accessContractRoute `yaml:"routes"`
}

type accessContractRoute struct {
	Method    string `yaml:"method"`
	Path      string `yaml:"path"`
	Stability string `yaml:"stability"`
	Kind      string `yaml:"kind"`
	Access    string `yaml:"access"`
}

const (
	markerUserUIAuth   = 418
	markerUserAPIAuth  = 419
	markerAdminUIAuth  = 420
	markerAdminRole    = 421
	markerAdminAPIAuth = 422
)

func loadAccessContract(t *testing.T) accessContract {
	t.Helper()

	data, err := os.ReadFile("../../contract/http.yaml")
	if err != nil {
		t.Fatalf("read contract/http.yaml: %v", err)
	}

	var contract accessContract
	if err := yaml.Unmarshal(data, &contract); err != nil {
		t.Fatalf("unmarshal contract/http.yaml: %v", err)
	}

	return contract
}

func TestRouteAccessContract(t *testing.T) {
	contract := loadAccessContract(t)
	router := newAccessContractTestRouter()

	for _, route := range contract.Routes {
		if route.Stability != "stable" {
			continue
		}

		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			req := httptest.NewRequest(route.Method, materializeRoutePath(route.Path), nil)
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			switch route.Access {
			case "public":
				assertNotAuthMarker(t, rr.Code)

			case "user":
				if strings.HasPrefix(route.Path, "/auth/api/") {
					if rr.Code != markerUserAPIAuth {
						t.Fatalf("expected user API auth marker %d, got %d", markerUserAPIAuth, rr.Code)
					}
				} else {
					if rr.Code != markerUserUIAuth {
						t.Fatalf("expected user UI auth marker %d, got %d", markerUserUIAuth, rr.Code)
					}
				}

			case "admin":
				if strings.HasPrefix(route.Path, "/auth/api/") {
					if rr.Code != markerAdminAPIAuth {
						t.Fatalf("expected admin API auth marker %d, got %d", markerAdminAPIAuth, rr.Code)
					}
				} else {
					if rr.Code != markerAdminUIAuth {
						t.Fatalf("expected admin UI auth marker %d, got %d", markerAdminUIAuth, rr.Code)
					}
				}

			default:
				t.Fatalf("unsupported access level %q for %s %s", route.Access, route.Method, route.Path)
			}
		})
	}
}

func newAccessContractTestRouter() chi.Router {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	pass := func(next http.Handler) http.Handler { return next }

	marker := func(status int, body string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, body, status)
			})
		}
	}

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
		RedirectIfAuthenticated: pass,
		ReturnTo:                pass,
		RequireCSRF:             pass,
		RequireAPICSRF:          pass,

		RequireAppAccessAuthWithRefresh:   marker(markerUserUIAuth, "user-ui-auth"),
		RequireAppAccessAuthAPI:           marker(markerUserAPIAuth, "user-api-auth"),
		RequireAdminAccessAuthWithRefresh: marker(markerAdminUIAuth, "admin-ui-auth"),
		RequireAdminAccessAuthAPI:         marker(markerAdminAPIAuth, "admin-api-auth"),
		RequireAdminRole:                  marker(markerAdminRole, "admin-role"),
	}

	r := chi.NewRouter()
	r.Use(middlewareRequestLoggerForAccessContract(cfg.Logger))
	registerRoutes(r, cfg, mw)

	return r
}

func materializeRoutePath(path string) string {
	path = strings.ReplaceAll(path, "{userID}", "11111111-1111-1111-1111-111111111111")
	return path
}

func assertNotAuthMarker(t *testing.T, code int) {
	t.Helper()

	switch code {
	case markerUserUIAuth, markerUserAPIAuth, markerAdminUIAuth, markerAdminRole, markerAdminAPIAuth:
		t.Fatalf("expected public route, but auth middleware marker intercepted with status %d", code)
	}
}

func middlewareRequestLoggerForAccessContract(logger *slog.Logger) func(http.Handler) http.Handler {
	return httpmiddleware.RequestLogger(logger)
}
