package bootstrap

import (
	"log/slog"

	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/logging"
)

func NewLogger(cfg *config.Config) (*slog.Logger, error) {
	return logging.New(cfg.Logging.Level)
}
