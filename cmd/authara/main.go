package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/authara-org/authara/internal/bootstrap"
)

var Version = "dev"

func main() {
	// Binary self-check for Docker HEALTHCHECK
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(0)
	}

	app, err := bootstrap.NewApp()
	if err != nil {
		log.Fatalf("startup failed: %v", err)
	}
	defer func() {
		if err := app.Close(); err != nil {
			app.Logger.Error("app close failed", "err", err)
		}
	}()

	app.Logger.Info("starting authara", "version", Version)

	server, err := bootstrap.NewHTTPServer(app, Version)
	if err != nil {
		log.Fatalf("build http server: %v", err)
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	app.StartBackgroundWorkers(ctx)

	go func() {
		app.Logger.Info("http server listening", "addr", app.Config.Values.HttpAddr)
		if err := server.Start(); err != nil {
			app.Logger.Error("http server stopped unexpectedly", "err", err)
			stop()
		}
	}()

	<-ctx.Done()

	app.Logger.Info("shutting down authara")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		app.Logger.Error("graceful shutdown failed", "err", err)
	}

	app.Logger.Info("authara stopped")
}
