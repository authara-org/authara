package bootstrap

import (
	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/email"
	"github.com/authara-org/authara/internal/logging"
)

func newEmailSender(cfg *config.Config, logger *logging.Logger) email.Sender {
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

func warnIfEmailDeliveryUnavailable(cfg *config.Config, logger *logging.Logger) {
	if cfg.Email.IsDeliverable() {
		return
	}

	logger.Warn(logging.EventConfigurationStatus,
		logging.ComponentField(logging.ComponentEmail),
		logging.OperationField(logging.OperationConfiguration),
		logging.OutcomeField(logging.OutcomeDegraded),
		logging.ErrorCodeField(logging.ErrorConfigurationInvalid),
	)
}
