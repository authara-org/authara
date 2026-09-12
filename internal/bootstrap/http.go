package bootstrap

import (
	"fmt"
	"time"

	cachepkg "github.com/authara-org/authara/internal/cache"
	"github.com/authara-org/authara/internal/features"
	httpserver "github.com/authara-org/authara/internal/http"
	"github.com/authara-org/authara/internal/http/handlers/api"
	"github.com/authara-org/authara/internal/http/handlers/internalapi"
	"github.com/authara-org/authara/internal/http/handlers/ui"
	"github.com/authara-org/authara/internal/http/kit/render"
	httpmiddleware "github.com/authara-org/authara/internal/http/middleware"
	"github.com/authara-org/authara/internal/oauth/google"
	"github.com/authara-org/authara/internal/ratelimiter"
	"github.com/authara-org/authara/internal/session/token"
)

const assetsManifestPath = "./internal/http/static/manifest.json"

func NewHTTPServer(app *App, version string) (*httpserver.Server, error) {
	enabledFeatures := features.Features{
		ChallengeEnabled:     app.Config.Challenge.Enabled,
		AllowlistEnabled:     app.Config.AccessPolicy.AllowedEmailEnabled,
		UsernameLoginEnabled: app.Config.Authentication.UsernameLoginEnabled,
	}

	mw := httpserver.Middlewares{
		RedirectIfAuthenticated: httpmiddleware.RedirectIfAuthenticated(app.Services.Session, time.Now),
		RequireAppAccessAuthAPI: httpmiddleware.RequireAPIAccessAuth(
			app.Services.Session,
			token.AudienceApp,
			time.Now,
		),
		RequireAppAccessAuthWithRefresh: httpmiddleware.RequireAccessAuthWithRefresh(
			app.Services.Session,
			token.AudienceApp,
			app.Config.Token.AccessTokenTTL,
			app.Config.Session.RefreshTokenTTL,
			time.Now,
		),
		RequireAdminAccessAuthAPI: httpmiddleware.RequireAPIAccessAuth(
			app.Services.Session,
			token.AudienceAdmin,
			time.Now,
		),
		RequireAdminAccessAuthWithRefresh: httpmiddleware.RequireAccessAuthWithRefresh(
			app.Services.Session,
			token.AudienceAdmin,
			app.Config.Token.AccessTokenTTL,
			app.Config.Session.RefreshTokenTTL,
			time.Now,
		),
		RequireOperatorAccessAuthAPI: httpmiddleware.RequireAPIAccessAuth(
			app.Services.Session,
			token.AudienceOperator,
			time.Now,
		),
		RequireOperatorAccessAuthWithRefresh: httpmiddleware.RequireAccessAuthWithRefresh(
			app.Services.Session,
			token.AudienceOperator,
			app.Config.Token.AccessTokenTTL,
			app.Config.Session.RefreshTokenTTL,
			time.Now,
		),
		RequireInternalAPIAuth: httpmiddleware.RequireInternalAPIAuth(app.Config.InternalAPI.Token),
		RequirePublicOrganizationManagement: httpmiddleware.RequirePublicOrganizationManagement(
			app.Config.Organization.PublicOrganizationManagementEnabled,
		),
		RequireAdminRole:          httpmiddleware.RequireAdmin,
		RequireOperatorRole:       httpmiddleware.RequireOperator,
		RequireCSRF:               httpmiddleware.RequireCSRF,
		RequireAPICSRF:            httpmiddleware.RequireAPICSRF,
		ReturnTo:                  httpmiddleware.ReturnToWithDefault(app.Config.UI.DefaultReturnTo),
		HTMX:                      httpmiddleware.HTMXMiddleware,
		RequireChallengeEnabled:   httpmiddleware.RequireChallengeEnabled(enabledFeatures.ChallengeEnabled),
		RequireAllowlistEnabled:   httpmiddleware.RequireAllowlistEnabled(enabledFeatures.AllowlistEnabled),
		OptionalAppAccessIdentity: httpmiddleware.OptionalAccessIdentity(app.Services.Session, token.AudienceApp, time.Now),
	}

	assets, err := render.LoadAssetsManifest(assetsManifestPath)
	if err != nil {
		return nil, fmt.Errorf("load assets manifest: %w", err)
	}
	renderer := render.New(assets, enabledFeatures.ChallengeEnabled)
	authLimiter := newAuthLimiter(app)
	googleClient := google.New(app.Config.OAuth.GoogleClientID)
	handlers := httpserver.Handlers{
		UI: ui.New(
			app.Services.Admin,
			app.Services.Auth,
			app.Services.Passkeys,
			app.Services.Session,
			app.Services.Organizations,
			app.Services.Challenge,
			enabledFeatures,
			app.Services.Verification,
			app.Services.EmailTemplates,
			app.Config,
			authLimiter,
			app.Logger,
			googleClient,
			app.Services.OAuthProviders,
			app.Config.Token.AccessTokenTTL,
			app.Config.Session.RefreshTokenTTL,
			renderer,
		),
		API: api.New(
			app.Services.Auth,
			app.Services.Passkeys,
			app.Services.Session,
			app.Services.Organizations,
			app.Services.Challenge,
			app.Services.Verification,
			authLimiter,
			app.Logger,
			googleClient,
			app.Services.OAuthProviders,
			enabledFeatures.ChallengeEnabled,
			enabledFeatures.UsernameLoginEnabled,
			app.Config.Token.AccessTokenTTL,
			app.Config.Session.RefreshTokenTTL,
		),
		InternalAPI: internalapi.New(
			app.Services.Auth,
			app.Services.Organizations,
			app.Config.Organization.PublicOrganizationManagementEnabled,
		),
	}

	server := httpserver.NewServer(httpserver.ServerConfig{
		Version:           version,
		Addr:              app.Config.Values.HttpAddr,
		Dev:               app.Config.Values.AppEnv == "dev",
		TrustProxyHeaders: app.Config.Values.TrustProxyHeaders,
		Logger:            app.Logger,
		Observability:     app.Observability,
		OAuthProviders:    app.Services.OAuthProviders,
		Handlers:          handlers,
	}, mw)

	return server, nil
}

func newAuthLimiter(app *App) ratelimiter.AuthLimiter {
	provider := func() ratelimiter.LimiterConfig { return newLimiterConfig(app) }

	if app.Config.Cache.Provider == "redis" {
		if counter, ok := app.Cache.(cachepkg.Counter); ok {
			return ratelimiter.NewCacheLimiterWithConfig(counter, provider)
		}

		app.Logger.Warn("configured cache does not support atomic counters; falling back to in-memory rate limiter")
	}

	return ratelimiter.NewInMemoryLimiterWithConfig(provider)
}

func newLimiterConfig(app *App) ratelimiter.LimiterConfig {
	if app.Config != nil {
		policy := app.Config.CurrentRateLimits()
		return ratelimiter.LimiterConfig{
			LoginIPLimit: policy.LoginIPLimit, LoginIPWindow: policy.LoginIPWindow,
			LoginEmailLimit: policy.LoginEmailLimit, LoginEmailWindow: policy.LoginEmailWindow,
			SignupIPLimit: policy.SignupIPLimit, SignupIPWindow: policy.SignupIPWindow,
			SignupEmailLimit: policy.SignupEmailLimit, SignupEmailWindow: policy.SignupEmailWindow,
			PasswordResetIPLimit: policy.PasswordResetIPLimit, PasswordResetIPWindow: policy.PasswordResetIPWindow,
			PasswordResetEmailLimit: policy.PasswordResetEmailLimit, PasswordResetEmailWindow: policy.PasswordResetEmailWindow,
			PasskeyLoginIPLimit: policy.PasskeyLoginIPLimit, PasskeyLoginIPWindow: policy.PasskeyLoginIPWindow,
			ChallengeVerifyIPLimit: policy.ChallengeVerifyIPLimit, ChallengeVerifyIPWindow: policy.ChallengeVerifyIPWindow,
			ChallengeResendIPLimit: policy.ChallengeResendIPLimit, ChallengeResendIPWindow: policy.ChallengeResendIPWindow,
			CleanupEvery: policy.CleanupEvery, MaxEntries: policy.MaxEntries,
		}
	}
	return ratelimiter.LimiterConfig{
		LoginIPLimit:     app.Config.RateLimit.LoginIPLimit,
		LoginIPWindow:    app.Config.RateLimit.LoginIPWindow,
		LoginEmailLimit:  app.Config.RateLimit.LoginEmailLimit,
		LoginEmailWindow: app.Config.RateLimit.LoginEmailWindow,

		SignupIPLimit:     app.Config.RateLimit.SignupIPLimit,
		SignupIPWindow:    app.Config.RateLimit.SignupIPWindow,
		SignupEmailLimit:  app.Config.RateLimit.SignupEmailLimit,
		SignupEmailWindow: app.Config.RateLimit.SignupEmailWindow,

		PasswordResetIPLimit:     app.Config.RateLimit.PasswordResetIPLimit,
		PasswordResetIPWindow:    app.Config.RateLimit.PasswordResetIPWindow,
		PasswordResetEmailLimit:  app.Config.RateLimit.PasswordResetEmailLimit,
		PasswordResetEmailWindow: app.Config.RateLimit.PasswordResetEmailWindow,

		PasskeyLoginIPLimit:  app.Config.RateLimit.PasskeyLoginIPLimit,
		PasskeyLoginIPWindow: app.Config.RateLimit.PasskeyLoginIPWindow,

		ChallengeVerifyIPLimit:  app.Config.RateLimit.ChallengeVerifyIPLimit,
		ChallengeVerifyIPWindow: app.Config.RateLimit.ChallengeVerifyIPWindow,

		ChallengeResendIPLimit:  app.Config.RateLimit.ChallengeResendIPLimit,
		ChallengeResendIPWindow: app.Config.RateLimit.ChallengeResendIPWindow,

		CleanupEvery: app.Config.RateLimit.CleanupEvery,
		MaxEntries:   app.Config.RateLimit.MaxEntries,
	}
}
