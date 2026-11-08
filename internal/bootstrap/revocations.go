package bootstrap

import (
	"time"

	"github.com/authara-org/authara/internal/cache"
	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/logging"
	"github.com/authara-org/authara/internal/session/token"
)

// AccessTokenRevocationMarkerTTL keeps markers long enough for any access
// token issued under the operator-configurable maximum lifetime.
func AccessTokenRevocationMarkerTTL(accessTokenTTL time.Duration) time.Duration {
	return max(accessTokenTTL, config.MaxAccessTokenTTL)
}

// NewAccessTokenRevocations binds token validation to the revocation guarantee
// selected at startup. The expiry-only mode intentionally has no online store.
func NewAccessTokenRevocations(
	cfg *config.Config,
	backend cache.Cache,
	ttlProvider func() time.Duration,
) *token.AccessTokenRevocations {
	if cfg.Cache.AccessTokenRevocationMode == config.AccessTokenRevocationModeExpiry {
		backend = nil
	}
	return token.NewAccessTokenRevocationsWithTTL(backend, ttlProvider)
}

func logAccessTokenRevocationGuarantee(cfg *config.Config, logger *logging.Logger) {
	if cfg.Cache.AccessTokenRevocationMode == config.AccessTokenRevocationModeImmediate {
		logger.Info(logging.EventConfigurationStatus,
			logging.ComponentField(logging.ComponentAuthentication),
			logging.OperationField(logging.OperationRevocation),
			logging.OutcomeField(logging.OutcomeSucceeded),
			logging.ModeField(logging.ModeImmediate),
		)
		return
	}

	logger.Warn(logging.EventConfigurationStatus,
		logging.ComponentField(logging.ComponentAuthentication),
		logging.OperationField(logging.OperationRevocation),
		logging.OutcomeField(logging.OutcomeDegraded),
		logging.ModeField(logging.ModeExpiry),
	)
}
