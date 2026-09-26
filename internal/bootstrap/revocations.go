package bootstrap

import (
	"log/slog"
	"time"

	"github.com/authara-org/authara/internal/cache"
	"github.com/authara-org/authara/internal/config"
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

func logAccessTokenRevocationGuarantee(cfg *config.Config, logger *slog.Logger) {
	if cfg.Cache.AccessTokenRevocationMode == config.AccessTokenRevocationModeImmediate {
		logger.Info(
			"access-token revocation guarantee enabled",
			"mode", config.AccessTokenRevocationModeImmediate,
			"cache_provider", cfg.Cache.Provider,
			"failure_semantics", "deny protected requests when the revocation store is unavailable",
		)
		return
	}

	logger.Warn(
		"access-token revocation is expiry-only",
		"mode", config.AccessTokenRevocationModeExpiry,
		"cache_provider", cfg.Cache.Provider,
		"maximum_residual_access", cfg.Token.AccessTokenTTL,
		"failure_semantics", "issued access tokens may remain valid after logout or authorization changes until expiry",
	)
}
