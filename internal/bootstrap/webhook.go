package bootstrap

import (
	"net/http"

	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/webhook"
)

func newWebhookPublisher(cfg *config.Config) webhook.Publisher {
	if !cfg.Webhook.Enabled() {
		return webhook.NoopPublisher{}
	}

	baseSender := webhook.NewSender(
		cfg.Webhook.URL,
		cfg.Webhook.Secret,
		&http.Client{Timeout: cfg.Webhook.Timeout},
	)

	return webhook.NewFilteringPublisher(
		baseSender,
		cfg.Webhook.EnabledEventSet,
	)
}
