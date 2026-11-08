package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/authara-org/authara/internal/bootstrap"
	"github.com/authara-org/authara/internal/logging"
)

const (
	shutdownTimeout = 10 * time.Second
)

func Run(version string) (retErr error) {
	app, err := bootstrap.NewApp(version)
	if err != nil {
		return fmt.Errorf("startup failed: %w", err)
	}
	closeApp := true
	appClosed := false
	defer func() {
		if closeApp && !appClosed {
			retErr = errors.Join(retErr, app.Close())
		}
	}()

	server, err := bootstrap.NewHTTPServer(app, version)
	if err != nil {
		return fmt.Errorf("build http server: %w", err)
	}
	listener, err := listenHTTP(app.Config.Values.HttpAddr)
	if err != nil {
		return err
	}

	app.Logger.Info(logging.EventProcessLifecycle,
		logging.ComponentField(logging.ComponentProcess),
		logging.OperationField(logging.OperationStartup),
		logging.OutcomeField(logging.OutcomeStarted),
	)
	app.Logger.Info(logging.EventProcessLifecycle,
		logging.ComponentField(logging.ComponentHTTP),
		logging.OperationField(logging.OperationServeHTTP),
		logging.OutcomeField(logging.OutcomeStarted),
	)

	signalCtx, stopSignals := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stopSignals()

	workers := app.StartBackgroundWorkers(context.Background())
	retErr = superviseLifecycle(
		signalCtx,
		stopSignals,
		app.Logger,
		server,
		workers,
		listener,
		shutdownTimeout,
	)
	if errors.Is(retErr, errShutdownTimeout) {
		// At the hard deadline the process must exit. Components that ignored
		// cancellation may still be using application resources, so do not race
		// them by closing the store or cache synchronously.
		closeApp = false
		return retErr
	}

	closeErr := app.Close()
	appClosed = true
	retErr = errors.Join(retErr, closeErr)
	app.Logger.Info(logging.EventProcessLifecycle,
		logging.ComponentField(logging.ComponentProcess),
		logging.OperationField(logging.OperationShutdown),
		logging.OutcomeField(logging.OutcomeStopped),
	)
	return retErr
}

func listenHTTP(address string) (net.Listener, error) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", address, err)
	}
	return listener, nil
}
