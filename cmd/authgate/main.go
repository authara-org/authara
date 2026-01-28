package main

import (
	"context"
	"encoding/base64"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alexlup06/authgate/internal/auth"
	"github.com/alexlup06/authgate/internal/config"
	httpserver "github.com/alexlup06/authgate/internal/http"
	"github.com/alexlup06/authgate/internal/http/providers/google"
	"github.com/alexlup06/authgate/internal/logging"
	"github.com/alexlup06/authgate/internal/session"
	"github.com/alexlup06/authgate/internal/session/token"
	"github.com/alexlup06/authgate/internal/store"
	"github.com/alexlup06/authgate/internal/store/schema"
	"github.com/alexlup06/authgate/internal/store/tx"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	logger, err := logging.New(cfg.Logging.Level)
	logger.Info("starting authgate")

	decodedKeys := make(map[string][]byte)

	for id, encoded := range cfg.Token.Keys {
		key, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			logger.Error("invalid JWT key", "key_id", id, "err", err)
			os.Exit(1)
		}
		decodedKeys[id] = key
	}

	keySet, err := token.NewKeySet(
		cfg.Token.ActiveKeyID,
		decodedKeys,
	)
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

	startupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	schemaVersion, err := store.CurrentSchemaVersion(startupCtx)
	if err != nil {
		logger.Error("failed to read schema version", "err", err)
		os.Exit(1)
	}

	if schemaVersion != schema.RequiredSchemaVersion {
		logger.Error(
			"database schema version mismatch",
			"current", schemaVersion,
			"required", schema.RequiredSchemaVersion,
		)
		os.Exit(1)
	}

	txManager := tx.New(store)

	accessTokenService := token.NewAccessTokenService(
		keySet,
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

	googleClient := google.New(cfg.Google.ClientID)

	server := httpserver.NewServer(httpserver.ServerConfig{
		Addr:            cfg.HTTP.Addr,
		Auth:            authService,
		Dev:             cfg.Dev,
		Session:         sessionService,
		Logger:          logger,
		Store:           store,
		Google:          googleClient,
		AccessTokenTTL:  cfg.Token.AccessTokenTTL,
		RefreshTokenTTL: cfg.Session.RefreshTokenTTL,
	})

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
