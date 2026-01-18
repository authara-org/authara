package http

import (
	"net/http"

	"github.com/alexlup06/authgate/internal/http/handlers"
	"github.com/go-chi/chi/v5"
)

func registerRoutes(r chi.Router, cfg Config) {
	// Health
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Auth UI + API
	r.Route("/auth", func(r chi.Router) {
		h := handlers.NewAuthHandler(cfg.Auth, cfg.Session, cfg.Google)

		r.Get("/login", h.LoginPage)
		r.Post("/login", h.LoginPost)
		r.Post("/logout", h.Logout)

		r.Get("/signup", h.SignupPage)
		r.Post("/signup", h.SignupPost)
	})

	handlers.RegisterStatic(r, handlers.StaticConfig{Dev: cfg.Dev})

	// // Session validation (for SDK / backends)
	// r.Get("/sessions/validate", func(w http.ResponseWriter, r *http.Request) {
	// 	user, err := cfg.Session.ValidateRequest(r)
	// 	if err != nil {
	// 		w.WriteHeader(http.StatusUnauthorized)
	// 		return
	// 	}
	// 	writeJSON(w, user)
	// })
}
