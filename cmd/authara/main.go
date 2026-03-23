package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/bootstrap"
	"github.com/authara-org/authara/internal/config"
	httpserver "github.com/authara-org/authara/internal/http"
	"github.com/authara-org/authara/internal/http/kit/csrf"
	"github.com/authara-org/authara/internal/http/kit/render"
	httpmiddleware "github.com/authara-org/authara/internal/http/middleware"
	"github.com/authara-org/authara/internal/logging"
	"github.com/authara-org/authara/internal/oauth"
	"github.com/authara-org/authara/internal/oauth/google"
	"github.com/authara-org/authara/internal/ratelimiter"
	"github.com/authara-org/authara/internal/session"
	"github.com/authara-org/authara/internal/session/token"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/store/schema"
	"github.com/authara-org/authara/internal/store/tx"
	"github.com/authara-org/authara/internal/webhook"
)

var Version = "dev"

func main() {
	// Binary self-check for Docker HEALTHCHECK
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(0)
	}

	// regular server
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	logger, err := logging.New(cfg.Logging.Level)
	if err != nil {
		logger.Error("invalid token key configuration", "err", err)
		os.Exit(1)
	}
	logger.Info("starting authara")

	store, err := store.New(store.Config{
		Host:     cfg.DB.Host,
		Port:     cfg.DB.Port,
		Username: cfg.DB.Username,
		Password: cfg.DB.Password,
		Database: cfg.DB.Database,
		Schema:   cfg.DB.Schema,
		Timezone: cfg.DB.Timezone,
		LogSql:   cfg.DB.LogSQL,
	})
	if err != nil {
		logger.Error("failed to connect to database", "err", err)
		os.Exit(1)
	}

	err = bootstrap.CheckSchemaVersion(store, schema.RequiredSchemaVersion)
	if err != nil {
		logger.Error("startup failed", "err", err)
		os.Exit(1)
	}

	txManager := tx.New(store)

	secure := cfg.Values.AppEnv == "prod"

	csrf.Configure(secure)
	session.Configure(secure)

	accessTokenService := token.NewAccessTokenService(
		cfg.Token.KeySet,
		cfg.Token.Issuer,
		cfg.Token.AccessTokenTTL,
	)

	var webhookPublisher webhook.Publisher = webhook.NoopPublisher{}

	if cfg.Webhook.Enabled() {
		baseSender := webhook.NewSender(
			cfg.Webhook.URL,
			cfg.Webhook.Secret,
			&http.Client{Timeout: cfg.Webhook.Timeout},
		)

		webhookPublisher = webhook.NewFilteringPublisher(
			baseSender,
			cfg.Webhook.EnabledEventSet,
		)
	}

	authService := auth.New(auth.Config{
		Store:            store,
		Tx:               txManager,
		WebhookPublisher: webhookPublisher,
		Logger:           logger,
	})

	sessionService := session.New(session.SessionConfig{
		Store:                store,
		Tx:                   txManager,
		AccessTokens:         accessTokenService,
		SessionTTL:           cfg.Session.SessionTTL,
		RefreshTokenTTL:      cfg.Session.RefreshTokenTTL,
		RefreshTokenRotation: cfg.Session.RefreshTokenRotation,
	})

	googleClient := google.New(cfg.OAuth.GoogleClientID)

	providers := []oauth.OAuthProvider{}
	for _, p := range cfg.OAuth.Providers {
		switch p {
		case string(oauth.GoogleOAuth):
			googleProvider := oauth.NewOAuthProvider(oauth.GoogleOAuth, cfg.OAuth.GoogleClientID, cfg.Values.PublicURL)
			providers = append(providers, googleProvider)

		}
	}
	oauthProviders := oauth.OAuthProviders{
		Providers: providers,
	}

	redirectIfAuthenticated := httpmiddleware.RedirectIfAuthenticated(sessionService, time.Now)
	requireAppAccessAuthAPI := httpmiddleware.RequireAPIAccessAuth(sessionService, token.AudienceApp, time.Now)
	requireAppAccessAuthWithRefresh := httpmiddleware.RequireAccessAuthWithRefresh(
		sessionService,
		token.AudienceApp,
		cfg.Token.AccessTokenTTL,
		cfg.Session.RefreshTokenTTL,
		time.Now,
	)
	requireAdminAccessAuthAPI := httpmiddleware.RequireAPIAccessAuth(sessionService, token.AudienceAdmin, time.Now)
	requireAdminAccessAuthWithRefresh := httpmiddleware.RequireAccessAuthWithRefresh(
		sessionService,
		token.AudienceAdmin,
		cfg.Token.AccessTokenTTL,
		cfg.Session.RefreshTokenTTL,
		time.Now,
	)
	requireAdminRole := httpmiddleware.RequireAdmin
	requireCSRF := httpmiddleware.RequireCSRF
	requireAPICSRF := httpmiddleware.RequireAPICSRF
	returnTo := httpmiddleware.ReturnTo

	mw := httpserver.Middlewares{
		RedirectIfAuthenticated:           redirectIfAuthenticated,
		RequireAppAccessAuthAPI:           requireAppAccessAuthAPI,
		RequireAppAccessAuthWithRefresh:   requireAppAccessAuthWithRefresh,
		RequireAdminAccessAuthAPI:         requireAdminAccessAuthAPI,
		RequireAdminAccessAuthWithRefresh: requireAdminAccessAuthWithRefresh,
		RequireAdminRole:                  requireAdminRole,
		RequireCSRF:                       requireCSRF,
		RequireAPICSRF:                    requireAPICSRF,
		ReturnTo:                          returnTo,
	}

	limiter := ratelimiter.NewInMemoryLimiter(ratelimiter.LimiterConfig{
		LoginIPLimit:     cfg.RateLimit.LoginIPLimit,
		LoginIPWindow:    cfg.RateLimit.LoginIPWindow,
		LoginEmailLimit:  cfg.RateLimit.LoginEmailLimit,
		LoginEmailWindow: cfg.RateLimit.LoginEmailWindow,

		SignupIPLimit:     cfg.RateLimit.SignupIPLimit,
		SignupIPWindow:    cfg.RateLimit.SignupIPWindow,
		SignupEmailLimit:  cfg.RateLimit.SignupEmailLimit,
		SignupEmailWindow: cfg.RateLimit.SignupEmailWindow,

		CleanupEvery: cfg.RateLimit.CleanupEvery,
		MaxEntries:   cfg.RateLimit.MaxEntries,
	})

	assets, err := render.LoadAssetsManifest("./internal/http/static/manifest.json")
	if err != nil {
		log.Fatal(err)
	}
	renderer := render.New(assets)

	server := httpserver.NewServer(httpserver.ServerConfig{
		Version:         Version,
		Addr:            cfg.Values.HttpAddr,
		Auth:            authService,
		Dev:             cfg.Values.AppEnv == "dev",
		Session:         sessionService,
		Logger:          logger,
		Store:           store,
		AuthLimiter:     limiter,
		Google:          googleClient,
		OAuthProviders:  oauthProviders,
		AccessTokenTTL:  cfg.Token.AccessTokenTTL,
		RefreshTokenTTL: cfg.Session.RefreshTokenTTL,
		Render:          renderer,
	}, mw)

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	sessionService.StartCleanupWorker(ctx, logger, 5*time.Minute)

	go func() {
		logger.Info("http server listening", "addr", cfg.Values.HttpAddr)
		if err := server.Start(); err != nil {
			logger.Error("http server stopped unexpectedly", "err", err)
			stop()
		}
	}()

	<-ctx.Done()

	logger.Info("shutting down authara")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "err", err)
	}

	logger.Info("authara stopped")
}
