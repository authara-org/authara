package http

import (
	"net/http"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/handlers/meta"
	httpmiddleware "github.com/authara-org/authara/internal/http/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(cfg ServerConfig, mw Middlewares) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	if cfg.TrustProxyHeaders {
		r.Use(middleware.RealIP)
	}
	r.Use(httpmiddleware.SecurityHeaders(httpmiddleware.SecurityHeadersConfig{
		AllowGoogleOAuth: hasOAuthProvider(cfg, domain.ProviderGoogle),
	}))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(httpmiddleware.RequestLogger(cfg.Logger))

	registerRoutes(r, cfg, mw)

	return r
}

func hasOAuthProvider(cfg ServerConfig, name domain.Provider) bool {
	for _, provider := range cfg.OAuthProviders.Providers {
		if provider.Name == name {
			return true
		}
	}

	return false
}

func registerRoutes(r chi.Router, cfg ServerConfig, mw Middlewares) {
	r.Group(func(r chi.Router) {
		r.Get("/auth/health", meta.Health)
		r.Get("/auth/version", meta.Version(cfg.Version))
	})

	uih := cfg.Handlers.UI
	apih := cfg.Handlers.API
	internalh := cfg.Handlers.InternalAPI

	r.Route("/auth", func(r chi.Router) {
		r.Use(mw.ReturnTo)

		// Auth
		r.Group(func(r chi.Router) {
			r.Use(mw.HTMX)

			// authara internals
			r.Get("/successful-deletion", uih.SuccessfulDeletionPage)

			r.Group(func(r chi.Router) {
				r.Use(mw.RedirectIfAuthenticated)

				r.Get("/login", uih.LoginPage)
				r.Get("/signup", uih.SignupPage)
				r.Get("/provider-links/confirm", uih.ProviderLinkConfirmPage)
			})

			r.Group(func(r chi.Router) {
				r.Use(mw.RequireCSRF)

				r.Post("/signup", uih.SignupPost)
				r.Post("/login", uih.LoginPost)
				r.Post("/invitations/signup", uih.InvitationSignupPost)
				r.Post("/invitations/login", uih.InvitationLoginPost)
				r.Post("/passkeys/authenticate/options", uih.PasskeyAuthenticateOptionsPost)
				r.Post("/passkeys/authenticate/finish", uih.PasskeyAuthenticateFinishPost)
				r.Post("/provider-links/confirm", uih.ProviderLinkConfirmPost)
				r.Post("/sessions/logout", uih.LogoutPost)
				r.Post("/sessions/refresh", uih.RefreshPost)
			})

			r.Group(func(r chi.Router) {
				r.Use(mw.OptionalAppAccessIdentity)

				r.Get("/invitations/accept", uih.InvitationAcceptPage)
				r.Get("/invitations/signup", uih.InvitationSignupPage)
				r.Get("/invitations/login", uih.InvitationLoginPage)
			})

			r.Group(func(r chi.Router) {
				r.Use(mw.RequireChallengeEnabled)

				// Public challenge pages/actions
				r.Group(func(r chi.Router) {
					r.Use(mw.OptionalAppAccessIdentity)
					r.Get("/password-reset", uih.PasswordResetPage)
					r.Get("/verify-challenge/{action}", uih.VerifyChallengePage)

					r.Group(func(r chi.Router) {
						r.Use(mw.RequireCSRF)

						r.Post("/password-reset", uih.PasswordResetRequestPost)
						r.Post("/verify-challenge/{action}", uih.VerifyChallengePost)
						r.Post("/resend-challenge", uih.ResendChallengePost)
					})
				})

				// Authenticated challenge-starting actions
				r.Group(func(r chi.Router) {
					r.Use(mw.RequireAppAccessAuthWithRefresh)
					r.Use(mw.RequireCSRF)

					r.Post("/email-change", uih.EmailChangeRequestPost)
				})
			})

			r.Route("/oauth", func(r chi.Router) {
				r.Use(mw.OptionalAppAccessIdentity)
				r.Use(mw.RequireCSRF)

				r.Post("/google/callback", uih.GoogleCallback)
			})

			// regular user
			r.Group(func(r chi.Router) {
				r.Use(mw.RequireAppAccessAuthWithRefresh)

				r.Get("/account", uih.AccountGet)
				r.Get("/passkeys/setup", uih.PasskeySetupPage)

				// password pages
				r.Get("/providers/password/add", uih.AddPasswordPage)
				r.Get("/providers/password/change", uih.ChangePasswordPage)

				r.Group(func(r chi.Router) {
					r.Use(mw.RequireCSRF)

					r.Post("/user/username", uih.ChangeUsernamePost)
					r.Post("/user/delete", uih.DeleteUser)
					r.Post("/invitations/accept", uih.InvitationAcceptPost)

					r.Post("/sessions/{sessionID}/revoke", uih.RevokeSessionPost)
					r.Post("/sessions/revoke-other", uih.RevokeOtherSessionsPost)

					r.Post("/providers/{provider}/unlink", uih.UnlinkProviderPost)
					r.Post("/providers/password/link", uih.PasswordLinkPost)
					r.Post("/providers/password/change", uih.PasswordChangePost)
					r.Post("/providers/{provider}/link/start", uih.ProviderLinkStartPost)
					r.Post("/passkeys/register/options", uih.PasskeyRegisterOptionsPost)
					r.Post("/passkeys/register/finish", uih.PasskeyRegisterFinishPost)
					r.Post("/passkeys/{id}/delete", uih.PasskeyDeletePost)
				})
			})

			// admin
			r.Group(func(r chi.Router) {
				r.Use(mw.RequireAdminAccessAuthWithRefresh)
				r.Use(mw.RequireAdminRole)

				r.Get("/admin", uih.AdminPage)

				// UI
				r.Route("/admin", func(r chi.Router) {
					r.Get("/", uih.AdminPage)
					r.Get("/users", uih.AdminUsersPage)
					r.Get("/users/search", uih.AdminUserSearchGet)
					r.Get("/users/{userID}", uih.AdminUserDetailPage)
					r.Get("/failures", uih.AdminFailuresPage)
					r.Get("/audit", uih.AdminAuditPage)

					r.Group(func(r chi.Router) {
						r.Use(mw.RequireAllowlistEnabled)

						r.Get("/allowlist", uih.AdminAllowlistPage)
						r.Get("/allowlist/results", uih.AdminAllowlistResultsGet)

						r.Group(func(r chi.Router) {
							r.Use(mw.RequireCSRF)

							r.Post("/allowlist", uih.AdminAllowlistCreatePost)
							r.Post("/allowlist/{emailID}/delete", uih.AdminAllowlistDeletePost)
						})
					})

					// API
					r.Group(func(r chi.Router) {
						r.Use(mw.RequireCSRF)

						r.Post("/users/{userID}/disable", uih.DisableUserPost)
						r.Post("/users/{userID}/enable", uih.EnableUserPost)
						r.Post("/users/{userID}/roles/admin/grant", uih.GrantAdminPost)
						r.Post("/users/{userID}/roles/admin/revoke", uih.RevokeAdminPost)
						r.Post("/users/{userID}/sessions/{sessionID}/revoke", uih.RevokeAdminUserSessionPost)
						r.Post("/users/{userID}/sessions/revoke-all", uih.RevokeAllAdminUserSessionsPost)
					})
				})
			})
		})

		// API
		r.Route("/api/v1", func(r chi.Router) {
			r.Get("/csrf", apih.CSRFGet)

			r.Group(func(r chi.Router) {
				r.Use(mw.RequireAPICSRF)

				r.Post("/login", apih.LoginPost)
				r.Post("/signup", apih.SignupPost)
				r.Post("/sessions/logout", apih.LogoutPost)
				r.Post("/sessions/refresh", apih.RefreshPost)

			})
			r.Post("/tokens/refresh", apih.TokenRefreshPost)

			r.Group(func(r chi.Router) {
				r.Use(mw.RequireAppAccessAuthAPI)

				r.Get("/user", apih.UserGet)
				r.Get("/organizations", apih.OrganizationsGet)
				r.Get("/organizations/current", apih.OrganizationCurrentGet)
				r.Get("/organizations/current/members", apih.OrganizationCurrentMembersGet)
				// r.Post("/user/username", apih.ChangeUsername)
				// r.Post("/user/delete", apih.DeleteUser)

				r.Group(func(r chi.Router) {
					r.Use(mw.RequireAPICSRF)
					r.Post("/organizations/{organizationID}/switch", apih.OrganizationSwitchPost)
				})
			})

			r.Route("/admin", func(r chi.Router) {
				r.Use(mw.RequireAdminAccessAuthAPI)
				r.Use(mw.RequireAdminRole)

			})

		})

		// Internal server-to-server API
		r.Route("/internal/v1", func(r chi.Router) {
			r.Use(mw.RequireInternalAPIAuth)

			r.Get("/capabilities", internalh.CapabilitiesGet)
			r.Post("/organizations", internalh.CreateOrganization)
			r.Get("/organizations/{organizationID}", internalh.GetOrganization)
			r.Patch("/organizations/{organizationID}", internalh.UpdateOrganization)
			r.Get("/organizations/{organizationID}/members", internalh.ListOrganizationMembers)
			r.Get("/organizations/{organizationID}/members/{userID}", internalh.GetOrganizationMember)
			r.Get("/organizations/{organizationID}/invitations", internalh.ListOrganizationInvitations)
			r.Post("/organizations/{organizationID}/invitations", internalh.CreateOrganizationInvitation)
			r.Get("/organizations/{organizationID}/invitations/{invitationID}", internalh.GetOrganizationInvitation)
			r.Post("/organizations/{organizationID}/invitations/{invitationID}/revoke", internalh.RevokeOrganizationInvitation)
			r.Post("/organizations/{organizationID}/invitations/{invitationID}/resend", internalh.ResendOrganizationInvitation)
			r.Get("/users/{userID}/memberships", internalh.ListUserMemberships)
		})
	})

	meta.RegisterStatic(r, meta.StaticConfig{Dev: cfg.Dev})
}
