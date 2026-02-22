package http

import (
	"github.com/alexlup06-authgate/authgate/internal/http/handlers"
	"github.com/alexlup06-authgate/authgate/internal/http/middleware"
	"github.com/go-chi/chi/v5"
)

func registerRoutes(r chi.Router, cfg ServerConfig, mw Middlewares) {

	r.Group(func(r chi.Router) {

		r.Get("/auth/health", handlers.Health)
		r.Get("/auth/version", handlers.Version(cfg.Version))

	})

	r.Route("/auth", func(r chi.Router) {

		h := handlers.NewAuthHandler(
			handlers.AuthHandlerConfig{
				AuthService:     cfg.Auth,
				SessionService:  cfg.Session,
				Limiter:         cfg.AuthLimiter,
				Logger:          cfg.Logger,
				Google:          cfg.Google,
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

		r.Route("/user", func(r chi.Router) {
			r.Use(mw.RequireAppAccessAuthWithRefresh)

			r.Get("/account", h.AccountGet)

			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireCSRF)

				r.Post("/username", h.ChangeUsernamePost)
			})

		})

		r.Route("/auth/admin", func(r chi.Router) {
			r.Use(mw.RequireAdminAccessAuthWithRefresh)
			r.Use(mw.RequireAdminRole)

			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireCSRF)

				r.Post("/users/{userID}/disable", h.DisableUserPost)
			})
		})

		r.Route("/api/v1", func(r chi.Router) {
			r.Use(mw.RequireAppAccessAuthAPI)

			r.Get("/user", h.UserGet)
		})

	})

	handlers.RegisterStatic(r, handlers.StaticConfig{Dev: cfg.Dev})
}
