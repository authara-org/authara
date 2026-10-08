package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/authara-org/authara/internal/webhook"
)

type BackgroundWorkers struct {
	app      *App
	failures chan error
	stopping atomic.Bool
}

func (a *App) StartBackgroundWorkers(ctx context.Context) *BackgroundWorkers {
	workers := &BackgroundWorkers{
		app:      a,
		failures: make(chan error, 5),
	}

	a.Config.StartReconciler(ctx)
	workers.monitor("runtime settings reconciler", a.Config.ReconcilerDone())

	a.Maintenance.Run(ctx)
	workers.monitor("maintenance coordinator", a.Maintenance.Done())
	if a.QueueMonitor != nil {
		a.QueueMonitor.Run(ctx)
		workers.monitor("queue metrics monitor", a.QueueMonitor.Done())
	}

	// Recovery and security flows enqueue email even when optional challenge
	// verification is disabled, so email delivery has its own lifecycle.
	a.Services.EmailWorker.Run(ctx)
	workers.monitor("email worker", a.Services.EmailWorker.Done())
	a.Logger.Info("email workers started",
		"worker_count", a.Config.Email.WorkerCount,
		"provider", a.Config.Email.Provider,
	)

	if a.Services.WebhookWorker != nil {
		a.Services.WebhookWorker.Run(ctx)
		workers.monitor("webhook worker", a.Services.WebhookWorker.Done())
		a.Logger.Info("webhook workers started",
			"worker_count", a.Config.Webhook.WorkerCount,
			"poll_interval", webhook.DeliveryPoll.String(),
		)
	}

	return workers
}

func (w *BackgroundWorkers) Errors() <-chan error {
	return w.failures
}

func (w *BackgroundWorkers) Shutdown(ctx context.Context) error {
	w.stopping.Store(true)

	type result struct {
		name string
		err  error
	}
	componentCount := 3
	if w.app.Services.WebhookWorker != nil {
		componentCount++
	}
	if w.app.QueueMonitor != nil {
		componentCount++
	}
	results := make(chan result, componentCount)
	shutdown := func(name string, fn func(context.Context) error) {
		go func() {
			results <- result{name: name, err: fn(ctx)}
		}()
	}

	shutdown("runtime settings reconciler", w.app.Config.ShutdownReconciler)
	shutdown("maintenance coordinator", w.app.Maintenance.Shutdown)
	if w.app.QueueMonitor != nil {
		shutdown("queue metrics monitor", w.app.QueueMonitor.Shutdown)
	}
	shutdown("email worker", w.app.Services.EmailWorker.Shutdown)
	if w.app.Services.WebhookWorker != nil {
		shutdown("webhook worker", w.app.Services.WebhookWorker.Shutdown)
	}

	var errs []error
	for range componentCount {
		select {
		case component := <-results:
			if component.err != nil {
				errs = append(errs, fmt.Errorf("stop %s: %w", component.name, component.err))
			}
		case <-ctx.Done():
			return errors.Join(append(errs, ctx.Err())...)
		}
	}
	return errors.Join(errs...)
}

func (w *BackgroundWorkers) monitor(name string, done <-chan struct{}) {
	go func() {
		<-done
		if w.stopping.Load() {
			return
		}
		select {
		case w.failures <- fmt.Errorf("%s stopped unexpectedly", name):
		default:
		}
	}()
}
