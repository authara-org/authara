package http

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/challenge"
	"github.com/authara-org/authara/internal/http/kit/render"
	"github.com/authara-org/authara/internal/oauth"
	"github.com/authara-org/authara/internal/oauth/google"
	"github.com/authara-org/authara/internal/ratelimiter"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/store"
)

type ServerConfig struct {
	Version          string
	Addr             string
	Dev              bool
	Auth             *auth.Service
	Session          *session.Service
	Challenge        *challenge.Service
	ChallengeEnabled bool
	Verification     *challenge.VerificationCodeService
	Logger           *slog.Logger
	Store            *store.Store
	AuthLimiter      ratelimiter.AuthLimiter
	Google           *google.Client
	OAuthProviders   oauth.OAuthProviders
	AccessTokenTTL   time.Duration
	RefreshTokenTTL  time.Duration
	Render           render.Renderer
}

type Middlewares struct {
	RedirectIfAuthenticated func(http.Handler) http.Handler

	RequireAppAccessAuthWithRefresh   func(http.Handler) http.Handler
	RequireAppAccessAuthAPI           func(http.Handler) http.Handler
	RequireAdminAccessAuthWithRefresh func(http.Handler) http.Handler
	RequireAdminAccessAuthAPI         func(http.Handler) http.Handler
	RequireAdminRole                  func(http.Handler) http.Handler

	RequireCSRF    func(http.Handler) http.Handler
	RequireAPICSRF func(http.Handler) http.Handler

	ReturnTo func(http.Handler) http.Handler
	HTMX     func(http.Handler) http.Handler
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
