package bootstrap

import (
	"fmt"
	"time"

	httpserver "github.com/authara-org/authara/internal/http"
	"github.com/authara-org/authara/internal/http/kit/render"
	httpmiddleware "github.com/authara-org/authara/internal/http/middleware"
	"github.com/authara-org/authara/internal/oauth/google"
	"github.com/authara-org/authara/internal/ratelimiter"
	"github.com/authara-org/authara/internal/session/token"
)

const assetsManifestPath = "./internal/http/static/manifest.json"

func NewHTTPServer(app *App, version string) (*httpserver.Server, error) {
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
		RequireAdminRole:          httpmiddleware.RequireAdmin,
		RequireCSRF:               httpmiddleware.RequireCSRF,
		RequireAPICSRF:            httpmiddleware.RequireAPICSRF,
		ReturnTo:                  httpmiddleware.ReturnTo,
		HTMX:                      httpmiddleware.HTMXMiddleware,
		RequireChallengeEnabled:   httpmiddleware.RequireChallengeEnabled(app.Config.Challenge.Enabled),
		OptionalAppAccessIdentity: httpmiddleware.OptionalAccessIdentity(app.Services.Session, token.AudienceApp, time.Now),
	}

	assets, err := render.LoadAssetsManifest(assetsManifestPath)
	if err != nil {
		return nil, fmt.Errorf("load assets manifest: %w", err)
	}
	renderer := render.New(assets, app.Config.Challenge.Enabled)

	server := httpserver.NewServer(httpserver.ServerConfig{
		Version:           version,
		Addr:              app.Config.Values.HttpAddr,
		Dev:               app.Config.Values.AppEnv == "dev",
		TrustProxyHeaders: app.Config.Values.TrustProxyHeaders,
		Auth:              app.Services.Auth,
		Session:           app.Services.Session,
		Challenge:         app.Services.Challenge,
		ChallengeEnabled:  app.Config.Challenge.Enabled,
		Verification:      app.Services.Verification,
		Logger:            app.Logger,
		Store:             app.Store,
		AuthLimiter:       newAuthLimiter(app),
		Google:            google.New(app.Config.OAuth.GoogleClientID),
		OAuthProviders:    app.Services.OAuthProviders,
		AccessTokenTTL:    app.Config.Token.AccessTokenTTL,
		RefreshTokenTTL:   app.Config.Session.RefreshTokenTTL,
		Render:            renderer,
	}, mw)

	return server, nil
}

func newAuthLimiter(app *App) ratelimiter.AuthLimiter {
	return ratelimiter.NewInMemoryLimiter(ratelimiter.LimiterConfig{
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

		ChallengeVerifyIPLimit:  app.Config.RateLimit.ChallengeVerifyIPLimit,
		ChallengeVerifyIPWindow: app.Config.RateLimit.ChallengeVerifyIPWindow,
		ChallengeVerifyIDLimit:  app.Config.RateLimit.ChallengeVerifyIDLimit,
		ChallengeVerifyIDWindow: app.Config.RateLimit.ChallengeVerifyIDWindow,

		ChallengeResendIPLimit:  app.Config.RateLimit.ChallengeResendIPLimit,
		ChallengeResendIPWindow: app.Config.RateLimit.ChallengeResendIPWindow,
		ChallengeResendIDLimit:  app.Config.RateLimit.ChallengeResendIDLimit,
		ChallengeResendIDWindow: app.Config.RateLimit.ChallengeResendIDWindow,

		CleanupEvery: app.Config.RateLimit.CleanupEvery,
		MaxEntries:   app.Config.RateLimit.MaxEntries,
	})
}
