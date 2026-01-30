package http

import (
	"net/http"

	"github.com/alexlup06-authgate/authgate/internal/http/handlers"
	"github.com/alexlup06-authgate/authgate/internal/http/middleware"
	"github.com/go-chi/chi/v5"
)

func registerRoutes(r chi.Router, cfg ServerConfig, redirectIfAuthenticated func(http.Handler) http.Handler) {
	healthHandler := handlers.NewHealthHandler(cfg.Store)
	r.Get("/auth/health", healthHandler.Health)

	r.Route("/auth", func(r chi.Router) {
		r.Use(middleware.CSRFOnGetMiddleware)

		h := handlers.NewAuthHandler(
			cfg.Auth,
			cfg.Session,
			cfg.Google,
			handlers.AuthHandlerConfig{
				AccessTokenTTL:  cfg.AccessTokenTTL,
				RefreshTokenTTL: cfg.RefreshTokenTTL,
			})

		r.Group(func(r chi.Router) {
			r.Use(redirectIfAuthenticated)

			r.Get("/login", h.LoginPage)
			r.Get("/signup", h.SignupPage)
		})

		r.Post("/signup", h.SignupPost)
		r.Post("/login", h.LoginPost)
		r.Post("/logout", h.LogoutPost)

		r.Post("/oauth/google/callback", h.GoogleCallback)

		r.Post("/refresh", h.RefreshPost)
	})

	handlers.RegisterStatic(r, handlers.StaticConfig{Dev: cfg.Dev})
}
