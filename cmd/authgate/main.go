package main

import (
	"context"
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

	authService := auth.New(auth.Config{
		Store: store,
		Tx:    txManager,
	})

	sessionService := session.New(session.Config{
		Store: store,
	})

	googleClient := google.New(cfg.Auth.GoogleClientID)

	server := httpserver.NewServer(httpserver.Config{
		Addr:    cfg.HTTP.Addr,
		Auth:    authService,
		Dev:     cfg.Dev,
		Session: sessionService,
		Logger:  logger,
		Google:  googleClient,
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
