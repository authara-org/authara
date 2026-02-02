package http

import (
	"github.com/alexlup06-authgate/authgate/internal/http/handlers"
	"github.com/alexlup06-authgate/authgate/internal/http/middleware"
	"github.com/go-chi/chi/v5"
)

func registerRoutes(r chi.Router, cfg ServerConfig, mw Middlewares) {

	r.Group(func(r chi.Router) {
		h := handlers.NewHealthHandler(cfg.Store)

		r.Get("/auth/health", h.Health)
	})

	r.Route("/auth", func(r chi.Router) {

		h := handlers.NewAuthHandler(
			cfg.Auth,
			cfg.Session,
			cfg.Google,
			handlers.AuthHandlerConfig{
				AccessTokenTTL:  cfg.AccessTokenTTL,
				RefreshTokenTTL: cfg.RefreshTokenTTL,
			})

		r.Group(func(r chi.Router) {
			r.Use(mw.RedirectIfAuthenticated)

			r.Get("/login", h.LoginPage)
			r.Get("/signup", h.SignupPage)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireCSRF)

			r.Post("/signup", h.SignupPost)
			r.Post("/login", h.LoginPost)
			r.Post("/logout", h.LogoutPost)
			r.Post("/refresh", h.RefreshPost)
		})

		r.Route("/oauth", func(r chi.Router) {
			r.Post("/google/callback", h.GoogleCallback)
		})

		r.Group(func(r chi.Router) {
			r.Use(mw.RequireAppAccessAuth)

			r.Get("/user", h.UserGet)
		})

		r.Route("/auth/admin", func(r chi.Router) {
			r.Use(mw.RequireAdminAccessAuth)
			r.Use(mw.RequireAdminRole)

			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireCSRF)

				r.Post("/users/{userID}/disable", h.DisableUserPost)
			})
		})

	})

	handlers.RegisterStatic(r, handlers.StaticConfig{Dev: cfg.Dev})
}
