package http

import (
	"net/http"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/http/handlers/meta"
	httpmiddleware "github.com/authara-org/authara/internal/http/middleware"
	openapicontract "github.com/authara-org/authara/internal/http/openapi"
	"github.com/authara-org/authara/internal/http/showcase"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(cfg ServerConfig, mw Middlewares) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	if cfg.TrustProxyHeaders {
		r.Use(middleware.RealIP)
	}
	if cfg.Observability != nil {
		r.Use(cfg.Observability.Middleware)
	}
	r.Use(httpmiddleware.SecurityHeaders(httpmiddleware.SecurityHeadersConfig{
		AllowGoogleOAuth: hasOAuthProvider(cfg, domain.ProviderGoogle),
		AllowShowcase:    cfg.Dev,
	}))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(httpmiddleware.RequestLogger(cfg.Logger))
	if !cfg.disableOpenAPIValidation {
		r.Use(openapicontract.ValidationMiddleware(cfg.Logger))
	}

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
	if cfg.Observability != nil {
		r.Get("/metrics", cfg.Observability.Handler().ServeHTTP)
	}

	r.Group(func(r chi.Router) {
		r.Get("/auth/health", meta.Health)
		r.Get("/auth/version", meta.Version(cfg.Version))
	})

	uih := cfg.Handlers.UI
	contracth := newOpenAPIServer(cfg.Handlers)

	r.Route("/auth", func(r chi.Router) {
		r.Use(mw.ReturnTo)
		if cfg.Dev {
			showcaseHandler := showcase.New(uih.Render)
			r.Get("/showcase", showcaseHandler.Gallery)
			r.Get("/showcase/pages/{slug}", showcaseHandler.Page)
		}

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

			// Public recovery and challenge actions. Password reset is always available.
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

			// Email-change completion must use the initiating authenticated session.
			r.Group(func(r chi.Router) {
				r.Use(mw.RequireAppAccessAuthWithRefresh)

				r.Get("/verify-challenge/email-change", uih.VerifyEmailChangeChallengePage)

				r.Group(func(r chi.Router) {
					r.Use(mw.RequireCSRF)
					r.Use(mw.RequireRecentAuthenticationUI)

					r.Post("/verify-challenge/email-change", uih.VerifyEmailChangeChallengePost)
				})
			})

			// Authenticated challenge-starting actions
			r.Group(func(r chi.Router) {
				r.Use(mw.RequireChallengeEnabled)
				r.Use(mw.RequireAppAccessAuthWithRefresh)
				r.Use(mw.RequireCSRF)
				r.Use(mw.RequireRecentAuthenticationUI)

				r.Post("/email-change", uih.EmailChangeRequestPost)
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
				r.Get("/reauthenticate", uih.ReauthenticatePage)
				r.Get("/reauthenticate/complete", uih.ReauthenticationCompletePage)

				r.Group(func(r chi.Router) {
					r.Use(mw.RequireCSRF)

					r.Post("/user/username", uih.ChangeUsernamePost)
					r.Post("/invitations/accept", uih.InvitationAcceptPost)

					r.Post("/sessions/{sessionID}/revoke", uih.RevokeSessionPost)
					r.Post("/sessions/revoke-other", uih.RevokeOtherSessionsPost)

					r.Post("/providers/password/change", uih.PasswordChangePost)
					r.Post("/reauthenticate/password", uih.ReauthenticatePasswordPost)
					r.Post("/reauthenticate/passkeys/options", uih.ReauthenticatePasskeyOptionsPost)
					r.Post("/reauthenticate/passkeys/finish", uih.ReauthenticatePasskeyFinishPost)

					r.Group(func(r chi.Router) {
						r.Use(mw.RequireRecentAuthenticationUI)
						r.Post("/user/delete", uih.DeleteUser)
						r.Post("/providers/{provider}/unlink", uih.UnlinkProviderPost)
						r.Post("/providers/password/link", uih.PasswordLinkPost)
						r.Post("/providers/{provider}/link/start", uih.ProviderLinkStartPost)
						r.Post("/passkeys/register/options", uih.PasskeyRegisterOptionsPost)
						r.Post("/passkeys/register/finish", uih.PasskeyRegisterFinishPost)
						r.Post("/passkeys/{id}/delete", uih.PasskeyDeletePost)
					})
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
					r.Get("/security-events", uih.AdminSecurityEventsPage)
					r.Get("/audit", uih.AdminAuditPage)

					r.Group(func(r chi.Router) {
						r.Use(mw.RequireAllowlistEnabled)

						r.Get("/allowlist", uih.AdminAllowlistPage)
						r.Get("/allowlist/results", uih.AdminAllowlistResultsGet)

						r.Group(func(r chi.Router) {
							r.Use(mw.RequireCSRF)
							r.Use(mw.RequireRecentAuthenticationUI)

							r.Post("/allowlist", uih.AdminAllowlistCreatePost)
							r.Post("/allowlist/{emailID}/delete", uih.AdminAllowlistDeletePost)
						})
					})

					// API
					r.Group(func(r chi.Router) {
						r.Use(mw.RequireCSRF)
						r.Use(mw.RequireRecentAuthenticationUI)

						r.Post("/users/{userID}/disable", uih.DisableUserPost)
						r.Post("/users/{userID}/enable", uih.EnableUserPost)
						r.Post("/users/{userID}/roles/admin/grant", uih.GrantAdminPost)
						r.Post("/users/{userID}/roles/admin/revoke", uih.RevokeAdminPost)
						r.Post("/users/{userID}/sessions/{sessionID}/revoke", uih.RevokeAdminUserSessionPost)
						r.Post("/users/{userID}/sessions/revoke-all", uih.RevokeAllAdminUserSessionsPost)
					})
				})
			})

			// operator
			r.Group(func(r chi.Router) {
				r.Use(mw.RequireOperatorAccessAuthWithRefresh)
				r.Use(mw.RequireOperatorRole)

				r.Get("/operator", uih.OperatorPage)
				r.Route("/operator", func(r chi.Router) {
					r.Get("/", uih.OperatorPage)
					r.Get("/audit", uih.OperatorAuditPage)
					r.Get("/settings", uih.OperatorSettingsPage)
					r.Get("/emails", uih.OperatorEmailTemplatesPage)
					r.Get("/emails/{templateKey}", uih.OperatorEmailTemplatePage)
					r.Get("/emails/{templateKey}/versions/{version}", uih.OperatorEmailTemplateVersionPage)

					r.Group(func(r chi.Router) {
						r.Use(httpmiddleware.LimitRequestBody(2 << 20))
						r.Use(mw.RequireCSRF)
						r.Post("/emails/{templateKey}/preview", uih.OperatorEmailTemplatePreviewPost)
					})

					r.Group(func(r chi.Router) {
						r.Use(httpmiddleware.LimitRequestBody(2 << 20))
						r.Use(mw.RequireCSRF)
						r.Use(mw.RequireRecentAuthenticationUI)
						r.Post("/emails/{templateKey}", uih.OperatorEmailTemplateSavePost)
						r.Post("/emails/{templateKey}/reset", uih.OperatorEmailTemplateResetPost)
						r.Post("/emails/{templateKey}/delivery", uih.OperatorEmailTemplateDeliveryPost)
					})

					r.Group(func(r chi.Router) {
						r.Use(httpmiddleware.LimitRequestBody(16 << 10))
						r.Use(mw.RequireCSRF)
						r.Use(mw.RequireRecentAuthenticationUI)
						r.Post("/settings/{settingKey}", uih.OperatorSettingSetPost)
						r.Post("/settings/{settingKey}/clear", uih.OperatorSettingClearPost)
					})
				})
			})
		})

		// API
		r.Route("/api/v1", func(r chi.Router) {
			r.Get("/csrf", contracth.GetCsrfToken)
			r.Get("/oauth/google/options", contracth.GetGoogleLoginOptions)
			r.Get("/invitations/preview", contracth.PreviewInvitation)

			r.Group(func(r chi.Router) {
				r.Use(mw.RequireAPICSRF)

				r.Post("/oauth/google", contracth.LoginWithGoogle)
				r.Post("/invitations/login", contracth.LoginAndAcceptInvitation)
				r.Post("/invitations/google", contracth.AuthenticateAndAcceptInvitationWithGoogle)
				r.Post("/provider-links/recovery/google", contracth.StartGoogleAccountRecoveryLink)
				r.Post("/provider-links/recovery/{linkID}/password", contracth.CompleteAccountRecoveryLinkWithPassword)
				r.Post("/provider-links/recovery/{linkID}/google", contracth.CompleteAccountRecoveryLinkWithGoogle)
				r.Post("/login", contracth.LoginWithPassword)
				r.Post("/signup/direct", contracth.SignupDirect)
				r.Post("/signup/challenges", contracth.StartSignupChallenge)
				r.Post("/signup/challenges/verify", contracth.VerifySignupChallenge)
				r.Post("/password-reset/challenges", contracth.StartPasswordResetChallenge)
				r.Post("/password-reset/challenges/verify", contracth.VerifyPasswordResetChallenge)
				r.Post("/challenges/resend", contracth.ResendChallenge)
				r.Post("/passkeys/authenticate/options", contracth.BeginPasskeyAuthentication)
				r.Post("/passkeys/authenticate/finish", contracth.FinishPasskeyAuthentication)
				r.Post("/sessions/logout", contracth.Logout)
				r.Post("/sessions/refresh", contracth.RefreshSession)

			})
			r.Post("/tokens/refresh", contracth.RefreshTokens)

			r.Group(func(r chi.Router) {
				r.Use(mw.RequireAppAccessAuthAPI)

				r.Get("/account", contracth.GetCurrentAccount)
				r.Get("/user", contracth.GetCurrentUser)
				r.Get("/capabilities", contracth.GetPublicCapabilities)
				r.Get("/organizations", contracth.ListCurrentUserOrganizations)
				r.Get("/organizations/current", contracth.GetCurrentOrganization)
				r.Get("/organizations/current/members", contracth.ListCurrentOrganizationMembers)
				r.Group(func(r chi.Router) {
					r.Use(mw.RequireAPICSRF)
					r.With(mw.RequireRecentAuthenticationAPI).Post("/reauthenticate/check", contracth.CheckRecentAuthentication)
					r.Post("/reauthenticate/password", contracth.ReauthenticateWithPassword)
					r.Post("/reauthenticate/google", contracth.ReauthenticateWithGoogle)
					r.Post("/reauthenticate/passkeys/options", contracth.BeginPasskeyReauthentication)
					r.Post("/reauthenticate/passkeys/finish", contracth.FinishPasskeyReauthentication)
					r.Post("/invitations/accept", contracth.AcceptInvitation)
					r.Patch("/account/username", contracth.ChangeCurrentUsername)
					r.Put("/account/password", contracth.ChangeCurrentUserPassword)
					r.Delete("/account/sessions/others", contracth.RevokeCurrentUserOtherSessions)
					r.Delete("/account/sessions/{sessionID}", contracth.RevokeCurrentUserSession)
					r.Post("/organizations/{organizationID}/switch", contracth.SwitchOrganization)

					r.Group(func(r chi.Router) {
						r.Use(mw.RequireRecentAuthenticationAPI)
						r.Post("/account/email-change/challenges", contracth.StartCurrentUserEmailChange)
						r.Post("/account/email-change/challenges/verify", contracth.VerifyCurrentUserEmailChange)
						r.Post("/account/password", contracth.AddCurrentUserPassword)
						r.Post("/account/auth-methods/google", contracth.LinkCurrentUserGoogle)
						r.Delete("/account/auth-methods/{provider}", contracth.UnlinkCurrentUserAuthMethod)
						r.Delete("/account/passkeys/{passkeyID}", contracth.DeleteCurrentUserPasskey)
						r.Put("/users/password", contracth.SetCurrentUserPassword)
						r.Post("/passkeys/register/options", contracth.BeginPasskeyRegistration)
						r.Post("/passkeys/register/finish", contracth.FinishPasskeyRegistration)
					})
				})

				// Optional direct organization management for authenticated apps.
				r.Group(func(r chi.Router) {
					r.Use(mw.RequirePublicOrganizationManagement)

					r.Get("/organizations/{organizationID}", contracth.GetPublicOrganization)
					r.Get("/organizations/{organizationID}/members", contracth.ListPublicOrganizationMembers)
					r.Get("/organizations/{organizationID}/members/{userID}", contracth.GetPublicOrganizationMember)
					r.Get("/organizations/{organizationID}/invitations", contracth.ListPublicOrganizationInvitations)
					r.Get("/organizations/{organizationID}/invitations/{invitationID}", contracth.GetPublicOrganizationInvitation)
					r.Get("/users/{userID}/memberships", contracth.ListPublicUserMemberships)

					r.Group(func(r chi.Router) {
						r.Use(mw.RequireAPICSRF)
						r.Use(mw.RequireRecentAuthenticationAPI)

						r.Patch("/organizations/{organizationID}", contracth.UpdatePublicOrganization)
						r.Patch("/organizations/{organizationID}/members/{userID}", contracth.UpdatePublicOrganizationMember)
						r.Post("/organizations/{organizationID}/invitations/{invitationID}/revoke", contracth.RevokePublicOrganizationInvitation)
					})
				})
			})

			r.Route("/admin", func(r chi.Router) {
				r.Use(mw.RequireAdminAccessAuthAPI)
				r.Use(mw.RequireAdminRole)

			})

		})

		// Internal server-to-server organization management.
		r.Route("/internal/v1", func(r chi.Router) {
			r.Use(mw.RequireInternalAPIAuth)

			r.Post("/organizations", contracth.CreateInternalOrganization)
			r.Delete("/organizations/{organizationID}", contracth.DeleteInternalOrganization)
			r.Patch("/organizations/{organizationID}/members/{userID}", contracth.UpdateInternalOrganizationMember)
			r.Delete("/organizations/{organizationID}/members/{userID}", contracth.RemoveInternalOrganizationMember)
			r.Post("/organizations/{organizationID}/ownership-transfer", contracth.TransferInternalOrganizationOwnership)
			r.Post("/organizations/{organizationID}/invitations", contracth.CreateInternalOrganizationInvitation)
			r.Post("/organizations/{organizationID}/invitations/{invitationID}/resend", contracth.ResendInternalOrganizationInvitation)

			r.Delete("/users/{userID}", contracth.DeleteInternalUser)
		})
	})

	meta.RegisterStatic(r, meta.StaticConfig{Dev: cfg.Dev})
}
