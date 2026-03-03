package http

import (
	authhandler "github.com/alexlup06-authgate/authgate/internal/http/handlers/auth"
	"github.com/alexlup06-authgate/authgate/internal/http/handlers/auth/api"
	"github.com/alexlup06-authgate/authgate/internal/http/handlers/auth/ui"
	"github.com/alexlup06-authgate/authgate/internal/http/handlers/meta"
	"github.com/go-chi/chi/v5"
)

func registerRoutes(r chi.Router, cfg ServerConfig, mw Middlewares) {
	r.Group(func(r chi.Router) {
		r.Get("/auth/health", meta.Health)
		r.Get("/auth/version", meta.Version(cfg.Version))
	})

	deps := authhandler.Deps{
		Auth:       cfg.Auth,
		Session:    cfg.Session,
		Limiter:    cfg.AuthLimiter,
		Logger:     cfg.Logger,
		Google:     cfg.Google,
		AccessTTL:  cfg.AccessTokenTTL,
		RefreshTTL: cfg.RefreshTokenTTL,
	}

	uih := ui.NewUIHandler(deps)
	apih := api.NewAPIHandler(deps)

	r.Route("/auth", func(r chi.Router) {
		r.Use(mw.ReturnTo)

		// Auth
		r.Group(func(r chi.Router) {

			r.Group(func(r chi.Router) {
				r.Use(mw.RedirectIfAuthenticated)

				r.Get("/login", uih.LoginPage)
				r.Get("/signup", uih.SignupPage)
			})

			r.Get("/successfull-deletion", uih.SuccessfullDeletionPage)

			r.Group(func(r chi.Router) {
				r.Use(mw.RequireCSRF)

				r.Post("/signup", uih.SignupPost)
				r.Post("/login", uih.LoginPost)
				r.Post("/sessions/logout", uih.LogoutPost)
				r.Post("/sessions/refresh", uih.RefreshPost)
			})

			r.Route("/oauth", func(r chi.Router) {
				r.Post("/google/callback", uih.GoogleCallback)
			})
		})

		// regular user
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireAppAccessAuthWithRefresh)

			r.Get("/account", uih.AccountGet)

			r.Group(func(r chi.Router) {
				r.Use(mw.RequireCSRF)

				r.Post("/user/username", uih.ChangeUsernamePost)
				r.Post("/user/delete", uih.DeleteUser)
			})
		})

		// admin
		r.Route("/admin", func(r chi.Router) {
			r.Use(mw.RequireAdminAccessAuthWithRefresh)
			r.Use(mw.RequireAdminRole)

			// UI
			r.Group(func(r chi.Router) {

			})

			// API
			r.Group(func(r chi.Router) {
				r.Use(mw.RequireCSRF)

				r.Post("/users/{userID}/disable", uih.DisableUserPost)
			})
		})
	})

	// API
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/csrf", apih.CSRFGet)

		r.Group(func(r chi.Router) {
			r.Use(mw.RequireCSRF)

			r.Post("/login", apih.LoginPost)
			r.Post("/signup", apih.SignupPost)
			r.Post("/sessions/logout", apih.LogoutPost)
			r.Post("/sessions/refresh", apih.RefreshPost)

		})

		r.Group(func(r chi.Router) {
			r.Use(mw.RequireAppAccessAuthAPI)

			r.Get("/user", apih.UserGet)
		})

		r.Route("/admin", func(r chi.Router) {
			r.Use(mw.RequireAppAccessAuthAPI)

		})

	})

	meta.RegisterStatic(r, meta.StaticConfig{Dev: cfg.Dev})
}
