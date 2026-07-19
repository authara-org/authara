package bootstrap

import (
	"context"
	"time"

	"github.com/authara-org/authara/internal/webhook"
)

func (a *App) StartBackgroundWorkers(ctx context.Context) {
	a.Services.Session.StartCleanupWorker(ctx, a.Logger, 5*time.Minute)
	a.Services.Admin.StartAuditCleanupWorker(ctx, a.Logger, 24*time.Hour)

	if a.Config.Challenge.Enabled {
		a.Services.EmailWorker.Run(ctx)
		a.Logger.Info("challenge email workers started",
			"worker_count", a.Config.Email.WorkerCount,
			"provider", a.Config.Email.Provider,
		)
	}

	if a.Services.WebhookWorker != nil {
		a.Services.WebhookWorker.Run(ctx)
		a.Logger.Info("webhook workers started",
			"worker_count", a.Config.Webhook.WorkerCount,
			"poll_interval", webhook.DeliveryPoll.String(),
		)
	}
}
