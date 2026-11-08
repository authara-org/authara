package email

import (
	"context"

	"github.com/authara-org/authara/internal/logging"
)

type NoopSenderConfig struct {
	Logger *logging.Logger
}

type NoopSender struct {
	logger *logging.Logger
}

func NewNoopSender(cfg NoopSenderConfig) *NoopSender {
	if cfg.Logger == nil {
		cfg.Logger = logging.Discard()
	}

	return &NoopSender{
		logger: cfg.Logger,
	}
}

func (s *NoopSender) Send(ctx context.Context, _ string, msg Message) error {
	// Recipients and rendered content may contain account PII or credentials.
	s.logger.Log(ctx, logging.LevelInfo, logging.EventWorkerOutcome,
		logging.WorkerField(logging.WorkerEmail),
		logging.OperationField(logging.OperationDeliver),
		logging.OutcomeField(logging.OutcomeSkipped),
	)

	return nil
}
