package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"
)

var errShutdownTimeout = errors.New("shutdown deadline exceeded")

type lifecycleServer interface {
	Serve(net.Listener) error
	Shutdown(context.Context) error
	Close() error
	SetReady(bool)
}

type lifecycleWorkers interface {
	Errors() <-chan error
	Shutdown(context.Context) error
}

func superviseLifecycle(
	signalCtx context.Context,
	stopSignals func(),
	logger *slog.Logger,
	server lifecycleServer,
	workers lifecycleWorkers,
	listener net.Listener,
	timeout time.Duration,
) error {
	serveResult := make(chan error, 1)
	go func() {
		serveResult <- server.Serve(listener)
	}()
	server.SetReady(true)

	var cause error
	serveReturned := false
	select {
	case <-signalCtx.Done():
	case err := <-serveResult:
		serveReturned = true
		if err == nil {
			cause = errors.New("http server stopped unexpectedly")
		} else {
			cause = fmt.Errorf("http server failed: %w", err)
		}
	case err := <-workers.Errors():
		if err == nil {
			cause = errors.New("background workers stopped unexpectedly")
		} else {
			cause = fmt.Errorf("background worker failed: %w", err)
		}
	}

	// Restore the default signal behavior while draining so a second signal
	// can force termination if an external dependency ignores cancellation.
	stopSignals()
	server.SetReady(false)
	logger.Info("shutting down authara")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	type result struct {
		name string
		err  error
	}
	results := make(chan result, 2)
	go func() { results <- result{name: "http server", err: server.Shutdown(shutdownCtx)} }()
	go func() { results <- result{name: "background workers", err: workers.Shutdown(shutdownCtx)} }()

	var shutdownErrs []error
	for range 2 {
		select {
		case component := <-results:
			if component.err != nil {
				shutdownErrs = append(shutdownErrs, fmt.Errorf("stop %s: %w", component.name, component.err))
			}
		case <-shutdownCtx.Done():
			closeErr := server.Close()
			return errors.Join(
				cause,
				errors.Join(shutdownErrs...),
				fmt.Errorf("%w: %v", errShutdownTimeout, shutdownCtx.Err()),
				closeErr,
			)
		}
	}

	if !serveReturned {
		select {
		case err := <-serveResult:
			if err != nil {
				shutdownErrs = append(shutdownErrs, fmt.Errorf("http server stopped: %w", err))
			}
		case <-shutdownCtx.Done():
			closeErr := server.Close()
			return errors.Join(
				cause,
				errors.Join(shutdownErrs...),
				fmt.Errorf("%w: %v", errShutdownTimeout, shutdownCtx.Err()),
				closeErr,
			)
		}
	}

	shutdownErr := errors.Join(shutdownErrs...)
	if shutdownCtx.Err() != nil || errors.Is(shutdownErr, context.DeadlineExceeded) {
		closeErr := server.Close()
		return errors.Join(
			cause,
			shutdownErr,
			fmt.Errorf("%w: %v", errShutdownTimeout, shutdownCtx.Err()),
			closeErr,
		)
	}
	return errors.Join(cause, shutdownErr)
}
