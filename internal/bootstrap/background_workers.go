package bootstrap

import (
	"context"
	"time"

	"github.com/authara-org/authara/internal/webhook"
)

func (a *App) StartBackgroundWorkers(ctx context.Context) {
	a.Config.StartReconciler(ctx)
	a.Services.Session.StartCleanupWorker(ctx, a.Logger, 5*time.Minute)
	a.Services.Admin.StartAuditCleanupWorker(ctx, a.Logger, 24*time.Hour)
	a.Services.SecurityEvents.StartCleanupWorker(ctx, a.Logger, 24*time.Hour)

	// Recovery and security flows enqueue email even when optional challenge
	// verification is disabled, so email delivery has its own lifecycle.
	a.Services.EmailWorker.Run(ctx)
	a.Logger.Info("email workers started",
		"worker_count", a.Config.Email.WorkerCount,
		"provider", a.Config.Email.Provider,
	)

	if a.Services.WebhookWorker != nil {
		a.Services.WebhookWorker.Run(ctx)
		a.Logger.Info("webhook workers started",
			"worker_count", a.Config.Webhook.WorkerCount,
			"poll_interval", webhook.DeliveryPoll.String(),
		)
	}
}
