package http

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/alexlup06-authgate/authgate/internal/auth"
	httpmiddleware "github.com/alexlup06-authgate/authgate/internal/http/middleware"
	"github.com/alexlup06-authgate/authgate/internal/oauth/google"
	"github.com/alexlup06-authgate/authgate/internal/ratelimit"
	"github.com/alexlup06-authgate/authgate/internal/session"
	"github.com/alexlup06-authgate/authgate/internal/store"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type ServerConfig struct {
	Version         string
	Addr            string
	Auth            *auth.Service
	Dev             bool
	Session         *session.Service
	Logger          *slog.Logger
	Store           *store.Store
	AuthLimiter     ratelimit.AuthLimiter
	Google          *google.Client
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

type Middlewares struct {
	RedirectIfAuthenticated func(http.Handler) http.Handler

	RequireAppAccessAuthWithRefresh   func(http.Handler) http.Handler
	RequireAppAccessAuthAPI           func(http.Handler) http.Handler
	RequireAdminAccessAuthWithRefresh func(http.Handler) http.Handler
	RequireAdminRole                  func(http.Handler) http.Handler

	RequireCSRF func(http.Handler) http.Handler

	ReturnTo func(http.Handler) http.Handler
}

type Server struct {
	httpServer *http.Server
}

func NewServer(cfg ServerConfig, mw Middlewares) *Server {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Use(httpmiddleware.RequestLogger(cfg.Logger))

	registerRoutes(r, cfg, mw)

	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      r,
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
