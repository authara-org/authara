package bootstrap

import (
	"net/url"
	"strings"

	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/logging"
)

func warnIfTransportMayBeInsecure(cfg *config.Config, logger *logging.Logger) {
	publicURL, err := url.Parse(cfg.Values.PublicURL)
	if err == nil && cfg.Values.AppEnv == "prod" && !strings.EqualFold(publicURL.Scheme, "https") {
		logger.Warn(logging.EventConfigurationStatus,
			logging.ComponentField(logging.ComponentHTTP),
			logging.OperationField(logging.OperationTransportSecurity),
			logging.OutcomeField(logging.OutcomeDegraded),
			logging.ErrorCodeField(logging.ErrorConfigurationInvalid),
		)
	}

	if cfg.Cache.Provider == "redis" {
		logger.Warn(logging.EventConfigurationStatus,
			logging.ComponentField(logging.ComponentCache),
			logging.OperationField(logging.OperationTransportSecurity),
			logging.OutcomeField(logging.OutcomeDegraded),
			logging.ErrorCodeField(logging.ErrorConfigurationInvalid),
		)
	}
}
