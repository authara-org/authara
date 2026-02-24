package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alexlup06-authgate/authgate/internal/auth"
	"github.com/alexlup06-authgate/authgate/internal/bootstrap"
	"github.com/alexlup06-authgate/authgate/internal/config"
	httpserver "github.com/alexlup06-authgate/authgate/internal/http"
	"github.com/alexlup06-authgate/authgate/internal/http/kit/csrf"
	httpmiddleware "github.com/alexlup06-authgate/authgate/internal/http/middleware"
	"github.com/alexlup06-authgate/authgate/internal/logging"
	"github.com/alexlup06-authgate/authgate/internal/oauth/google"
	"github.com/alexlup06-authgate/authgate/internal/ratelimit"
	"github.com/alexlup06-authgate/authgate/internal/session"
	"github.com/alexlup06-authgate/authgate/internal/session/token"
	"github.com/alexlup06-authgate/authgate/internal/store"
	"github.com/alexlup06-authgate/authgate/internal/store/schema"
	"github.com/alexlup06-authgate/authgate/internal/store/tx"
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
	logger.Info("starting authgate")
	if err != nil {
		logger.Error("invalid token key configuration", "err", err)
		os.Exit(1)
	}

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

	authService := auth.New(auth.Config{
		Store: store,
		Tx:    txManager,
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

	redirectIfAuthenticated := httpmiddleware.RedirectIfAuthenticated(sessionService, time.Now)
	requireAppAccessAuthAPI := httpmiddleware.RequireAccessAuth(sessionService, token.AudienceApp, time.Now)
	requireAppAccessAuthWithRefresh := httpmiddleware.RequireAccessAuthWithRefresh(
		sessionService,
		token.AudienceApp,
		cfg.Token.AccessTokenTTL,
		cfg.Session.RefreshTokenTTL,
		time.Now,
	)
	requireAdminAccessAuthWithRefresh := httpmiddleware.RequireAccessAuthWithRefresh(
		sessionService,
		token.AudienceAdmin,
		cfg.Token.AccessTokenTTL,
		cfg.Session.RefreshTokenTTL,
		time.Now,
	)
	requireAdminRole := httpmiddleware.RequireAdmin
	requireCSRF := httpmiddleware.RequireCSRF
	returnTo := httpmiddleware.ReturnTo

	mw := httpserver.Middlewares{
		RedirectIfAuthenticated:           redirectIfAuthenticated,
		RequireAppAccessAuthAPI:           requireAppAccessAuthAPI,
		RequireAppAccessAuthWithRefresh:   requireAppAccessAuthWithRefresh,
		RequireAdminAccessAuthWithRefresh: requireAdminAccessAuthWithRefresh,
		RequireAdminRole:                  requireAdminRole,
		RequireCSRF:                       requireCSRF,
		ReturnTo:                          returnTo,
	}

	limiter := ratelimit.NewInMemoryLimiter(ratelimit.LimiterConfig{
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

	server := httpserver.NewServer(httpserver.ServerConfig{
		Version:         Version,
		Addr:            cfg.HTTP.Addr,
		Auth:            authService,
		Dev:             cfg.Values.AppEnv == "dev",
		Session:         sessionService,
		Logger:          logger,
		Store:           store,
		AuthLimiter:     limiter,
		Google:          googleClient,
		AccessTokenTTL:  cfg.Token.AccessTokenTTL,
		RefreshTokenTTL: cfg.Session.RefreshTokenTTL,
	}, mw)

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	go func() {
		logger.Info("http server listening", "addr", cfg.HTTP.Addr)
		if err := server.Start(); err != nil {
			logger.Error("http server stopped unexpectedly", "err", err)
			stop()
		}
	}()

	<-ctx.Done()

	logger.Info("shutting down authgate")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "err", err)
	}

	logger.Info("authgate stopped")
}
