package http

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/alexlup06/authgate/internal/auth"
	httpmiddleware "github.com/alexlup06/authgate/internal/http/middleware"
	"github.com/alexlup06/authgate/internal/http/providers/google"
	"github.com/alexlup06/authgate/internal/session"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Server struct {
	httpServer *http.Server
}

type Config struct {
	Addr    string
	Auth    *auth.Service
	Dev     bool
	Session *session.Service
	Logger  *slog.Logger
	Google  *google.Client
}

func NewServer(cfg Config) *Server {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	// Logging middleware (yours)
	r.Use(httpmiddleware.RequestLogger(cfg.Logger))

	registerRoutes(r, cfg)

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
