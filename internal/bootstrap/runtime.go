package bootstrap

import (
	"github.com/authara-org/authara/internal/config"
	"github.com/authara-org/authara/internal/http/kit/csrf"
	"github.com/authara-org/authara/internal/http/kit/oauthstate"
	"github.com/authara-org/authara/internal/session"
)

func configureRuntime(cfg *config.Config) {
	secure := cfg.Values.AppEnv == "prod"

	csrf.Configure(secure)
	oauthstate.Configure(secure)
	session.Configure(secure)
}
