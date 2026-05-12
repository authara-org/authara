package bootstrap

import (
	"log/slog"

	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/email"
)

func newEmailSender(cfg *config.Config, logger *slog.Logger) email.Sender {
	switch cfg.Email.Provider {
	case "smtp":
		return email.NewSMTPSender(
			cfg.Email.SMTPHost,
			cfg.Email.SMTPPort,
			cfg.Email.SMTPUsername,
			cfg.Email.SMTPPassword,
			cfg.Email.From,
			cfg.Email.SMTPTLS,
			cfg.Email.SMTPTimeout,
		)
	default:
		return email.NewNoopSender(email.NoopSenderConfig{
			Logger: logger,
		})
	}
}
