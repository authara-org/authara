package http

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestShowcaseRoutesExistOnlyInDevelopment(t *testing.T) {
	for _, tc := range []struct {
		name string
		dev  bool
		want int
	}{
		{name: "development", dev: true, want: http.StatusOK},
		{name: "production", dev: false, want: http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			cfg := ServerConfig{
				Dev:      tc.dev,
				Logger:   logger,
				Handlers: newTestHandlers(logger, nil),
			}
			router := NewRouter(cfg, passThroughMiddlewares())
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/auth/showcase", nil))
			if recorder.Code != tc.want {
				t.Fatalf("status = %d, want %d; body = %s", recorder.Code, tc.want, recorder.Body.String())
			}
		})
	}
}

func passThroughMiddlewares() Middlewares {
	pass := func(next http.Handler) http.Handler { return next }
	return Middlewares{
		RedirectIfAuthenticated:         pass,
		RequireAppAccessAuthWithRefresh: pass, RequireAppAccessAuthAPI: pass,
		RequireAdminAccessAuthWithRefresh: pass, RequireAdminAccessAuthAPI: pass,
		RequireOperatorAccessAuthWithRefresh: pass, RequireOperatorAccessAuthAPI: pass,
		RequireRecentAuthenticationUI: pass, RequireRecentAuthenticationAPI: pass,
		RequireInternalAPIAuth: pass, RequirePublicOrganizationManagement: pass,
		RequireAdminRole: pass, RequireOperatorRole: pass,
		RequireCSRF: pass, RequireAPICSRF: pass,
		ReturnTo: pass, HTMX: pass, RequireChallengeEnabled: pass,
		RequireAllowlistEnabled: pass, OptionalAppAccessIdentity: pass,
	}
}
