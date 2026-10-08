package http

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"time"

	"github.com/authara-org/authara/internal/http/handlers/api"
	"github.com/authara-org/authara/internal/http/handlers/internalapi"
	"github.com/authara-org/authara/internal/http/handlers/meta"
	"github.com/authara-org/authara/internal/http/handlers/ui"
	"github.com/authara-org/authara/internal/oauth"
	"github.com/authara-org/authara/internal/observability"
)

type ServerConfig struct {
	Version           string
	Addr              string
	Dev               bool
	TrustProxyHeaders bool
	TrustedProxyCIDRs []netip.Prefix
	Logger            *slog.Logger
	Observability     *observability.Service
	OAuthProviders    oauth.OAuthProviders
	Handlers          Handlers
	Readiness         *meta.Readiness

	disableOpenAPIValidation        bool
	strictOpenAPIResponseValidation bool
}

type Handlers struct {
	UI          *ui.UIHandler
	API         *api.APIHandler
	InternalAPI *internalapi.Handler
}

type Middlewares struct {
	RedirectIfAuthenticated func(http.Handler) http.Handler

	RequireAppAccessAuthWithRefresh      func(http.Handler) http.Handler
	RequireAppAccessAuthAPI              func(http.Handler) http.Handler
	RequireAdminAccessAuthWithRefresh    func(http.Handler) http.Handler
	RequireAdminAccessAuthAPI            func(http.Handler) http.Handler
	RequireOperatorAccessAuthWithRefresh func(http.Handler) http.Handler
	RequireOperatorAccessAuthAPI         func(http.Handler) http.Handler
	RequireAppVerifiedEmailUI            func(http.Handler) http.Handler
	RequireAppVerifiedEmailAPI           func(http.Handler) http.Handler
	RequireAdminVerifiedEmailUI          func(http.Handler) http.Handler
	RequireOperatorVerifiedEmailUI       func(http.Handler) http.Handler
	RequireRecentAuthenticationUI        func(http.Handler) http.Handler
	RequireRecentAuthenticationAPI       func(http.Handler) http.Handler
	RequireInternalAPIAuth               func(http.Handler) http.Handler
	RequirePublicOrganizationManagement  func(http.Handler) http.Handler
	RequireAdminRole                     func(http.Handler) http.Handler
	RequireOperatorRole                  func(http.Handler) http.Handler

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
	readiness  *meta.Readiness
}

func NewServer(cfg ServerConfig, mw Middlewares) *Server {
	readiness := cfg.Readiness
	if readiness == nil {
		readiness = meta.NewReadiness(false)
		cfg.Readiness = readiness
	}
	handler := NewRouter(cfg, mw)

	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return &Server{httpServer: srv, readiness: readiness}
}

func (s *Server) Serve(listener net.Listener) error {
	err := s.httpServer.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) Close() error {
	return s.httpServer.Close()
}

func (s *Server) SetReady(ready bool) {
	s.readiness.Set(ready)
}
