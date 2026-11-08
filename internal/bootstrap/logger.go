package bootstrap

import (
	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/logging"
)

func NewLogger(cfg *config.Config, version string) (*logging.Logger, error) {
	return logging.New(logging.Config{
		Level:       cfg.Logging.Level,
		Environment: cfg.Logging.Environment,
		Version:     version,
	})
}
