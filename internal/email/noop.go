package email

import (
	"context"
	"log/slog"
)

type NoopSenderConfig struct {
	Logger *slog.Logger
}

type NoopSender struct {
	logger *slog.Logger
}

func NewNoopSender(cfg NoopSenderConfig) *NoopSender {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	return &NoopSender{
		logger: cfg.Logger,
	}
}

func (s *NoopSender) Send(ctx context.Context, to string, msg Message) error {
	s.logger.InfoContext(ctx, "email noop send",
		"to", to,
		"subject", msg.Subject,
		"has_text", msg.Text != "",
		"has_html", msg.HTML != "",
	)

	return nil
}
