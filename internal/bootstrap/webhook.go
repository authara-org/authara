package bootstrap

import (
	"log/slog"
	"net/http"

	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/webhook"
)

func newWebhookPublisher(cfg *config.Config, store *store.Store) webhook.Publisher {
	if !cfg.Webhook.Enabled() {
		return webhook.NoopPublisher{}
	}

	return webhook.NewFilteringPublisher(
		webhook.NewQueuePublisher(store),
		cfg.Webhook.EnabledEventSet,
	)
}

func newWebhookWorker(cfg *config.Config, store *store.Store, logger *slog.Logger) *webhook.Worker {
	if !cfg.Webhook.Enabled() {
		return nil
	}

	sender := webhook.NewSender(
		cfg.Webhook.URL,
		cfg.Webhook.Secret,
		&http.Client{Timeout: cfg.Webhook.Timeout},
	)
	return webhook.NewWorker(
		store,
		sender,
		logger,
		webhook.WorkerConfig{
			WorkerCount:          cfg.Webhook.WorkerCount,
			PollInterval:         webhook.DeliveryPoll,
			MaxDeliveryAttempts:  cfg.Webhook.MaxDeliveryAttempts,
			ProcessingStaleAfter: cfg.Webhook.ProcessingStaleAfter,
			StaleReaperInterval:  cfg.Webhook.StaleReaperInterval,
			DeliveredRetention:   cfg.Webhook.DeliveredRetention,
			FailedRetention:      cfg.Webhook.FailedRetention,
			CleanupInterval:      cfg.Webhook.CleanupInterval,
			MaintenanceBatchSize: cfg.Webhook.MaintenanceBatchSize,
		},
	)
}
