package http

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/authara-org/authara/internal/http/handlers/api"
	"github.com/authara-org/authara/internal/http/handlers/internalapi"
	"github.com/authara-org/authara/internal/http/handlers/ui"
	"github.com/authara-org/authara/internal/oauth"
)

type ServerConfig struct {
	Version           string
	Addr              string
	Dev               bool
	TrustProxyHeaders bool
	Logger            *slog.Logger
	OAuthProviders    oauth.OAuthProviders
	Handlers          Handlers
}

type Handlers struct {
	UI          *ui.UIHandler
	API         *api.APIHandler
	InternalAPI *internalapi.Handler
}

type Middlewares struct {
	RedirectIfAuthenticated func(http.Handler) http.Handler

	RequireAppAccessAuthWithRefresh   func(http.Handler) http.Handler
	RequireAppAccessAuthAPI           func(http.Handler) http.Handler
	RequireAdminAccessAuthWithRefresh func(http.Handler) http.Handler
	RequireAdminAccessAuthAPI         func(http.Handler) http.Handler
	RequireInternalAPIAuth            func(http.Handler) http.Handler
	RequireAdminRole                  func(http.Handler) http.Handler

	RequireCSRF    func(http.Handler) http.Handler
	RequireAPICSRF func(http.Handler) http.Handler

	ReturnTo                  func(http.Handler) http.Handler
	HTMX                      func(http.Handler) http.Handler
	RequireChallengeEnabled   func(http.Handler) http.Handler
	RequireAllowlistEnabled   func(http.Handler) http.Handler
	OptionalAppAccessIdentity func(http.Handler) http.Handler
}

type Server struct {
	httpServer *http.Server
}

func NewServer(cfg ServerConfig, mw Middlewares) *Server {
	handler := NewRouter(cfg, mw)

	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return &Server{httpServer: srv}
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
