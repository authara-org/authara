package http

import (
	"net/http"
	"time"

	authhandler "github.com/authara-org/authara/internal/http/handlers/auth"
	"github.com/authara-org/authara/internal/http/handlers/auth/api"
	"github.com/authara-org/authara/internal/http/handlers/auth/ui"
	"github.com/authara-org/authara/internal/http/handlers/meta"
	httpmiddleware "github.com/authara-org/authara/internal/http/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(cfg ServerConfig, mw Middlewares) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(httpmiddleware.RequestLogger(cfg.Logger))

	registerRoutes(r, cfg, mw)

	return r
}

func registerRoutes(r chi.Router, cfg ServerConfig, mw Middlewares) {
	r.Group(func(r chi.Router) {
		r.Get("/auth/health", meta.Health)
		r.Get("/auth/version", meta.Version(cfg.Version))
	})

	deps := authhandler.Deps{
		Auth:             cfg.Auth,
		Session:          cfg.Session,
		Limiter:          cfg.AuthLimiter,
		Logger:           cfg.Logger,
		Google:           cfg.Google,
		OAuthProviders:   cfg.OAuthProviders,
		AccessTTL:        cfg.AccessTokenTTL,
		RefreshTTL:       cfg.RefreshTokenTTL,
		Render:           cfg.Render,
		Challenge:        cfg.Challenge,
		ChallengeEnabled: cfg.ChallengeEnabled,
		Verification:     cfg.Verification,
	}

	uih := ui.NewUIHandler(deps)
	apih := api.NewAPIHandler(deps)

	r.Route("/auth", func(r chi.Router) {
		r.Use(mw.ReturnTo)

		// Auth
		r.Group(func(r chi.Router) {
			r.Use(mw.HTMX)

			// authara internals
			r.Get("/verify-challenge", uih.VerifyChallengePage)
			r.Get("/successfull-deletion", uih.SuccessfullDeletionPage)

			r.Group(func(r chi.Router) {
				r.Use(mw.RedirectIfAuthenticated)

				r.Get("/login", uih.LoginPage)
				r.Get("/signup", uih.SignupPage)
			})

			r.Group(func(r chi.Router) {
				r.Use(mw.RequireCSRF)

				r.Post("/signup", uih.SignupPost)
				r.Post("/login", uih.LoginPost)
				r.Post("/verify-challenge", uih.VerifyChallengePost)
				r.Post("/resend-challenge", uih.ResendChallengePost)
				r.Post("/sessions/logout", uih.LogoutPost)
				r.Post("/sessions/refresh", uih.RefreshPost)
			})

			r.Route("/oauth", func(r chi.Router) {
				r.Post("/google/callback", uih.GoogleCallback)
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
					r.Get("/", uih.AdminPage)
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
			// r.Get("/csrf", apih.CSRFGet)

			r.Group(func(r chi.Router) {
				r.Use(mw.RequireAPICSRF)

				// r.Post("/login", apih.LoginPost)
				// r.Post("/signup", apih.SignupPost)
				r.Post("/sessions/logout", apih.LogoutPost)
				r.Post("/sessions/refresh", apih.RefreshPost)

			})

			r.Group(func(r chi.Router) {
				r.Use(mw.RequireAppAccessAuthAPI)

				r.Get("/user", apih.UserGet)
				// r.Post("/user/username", apih.ChangeUsername)
				// r.Post("/user/delete", apih.DeleteUser)
			})

			r.Route("/admin", func(r chi.Router) {
				r.Use(mw.RequireAdminAccessAuthAPI)

			})

		})
	})

	meta.RegisterStatic(r, meta.StaticConfig{Dev: cfg.Dev})
}
