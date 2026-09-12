package bootstrap

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/webhook"
)

func newWebhookPublisher(cfg *config.Service, store *store.Store) webhook.Publisher {
	if !cfg.Webhook.Enabled() {
		return webhook.NoopPublisher{}
	}

	return webhook.NewFilteringPublisherWithPolicy(
		webhook.NewQueuePublisher(store),
		func() []string { return cfg.CurrentWebhook().EnabledEvents },
	)
}

func newWebhookWorker(
	cfg *config.Service,
	store *store.Store,
	logger *slog.Logger,
	metrics webhook.WorkerMetrics,
) *webhook.Worker {
	if !cfg.Webhook.Enabled() {
		return nil
	}

	sender := webhook.NewSenderWithTimeout(
		cfg.Webhook.URL,
		cfg.Webhook.Secret,
		&http.Client{},
		func() time.Duration { return cfg.CurrentWebhook().Timeout },
	)
	return webhook.NewWorker(
		store,
		sender,
		logger,
		webhook.WorkerConfig{
			WorkerCount:         cfg.Webhook.WorkerCount,
			PollInterval:        webhook.DeliveryPoll,
			StaleReaperInterval: cfg.Webhook.StaleReaperInterval,
			CleanupInterval:     cfg.Webhook.CleanupInterval,
			Metrics:             metrics,
			Policy: func() webhook.WorkerPolicy {
				policy := cfg.CurrentWebhook()
				return webhook.WorkerPolicy{
					MaxDeliveryAttempts: policy.MaxDeliveryAttempts, ProcessingStaleAfter: policy.ProcessingStaleAfter,
					DeliveredRetention: policy.DeliveredRetention, FailedRetention: policy.FailedRetention,
					MaintenanceBatchSize: policy.MaintenanceBatchSize,
				}
			},
		},
	)
}
