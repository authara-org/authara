package bootstrap

import "github.com/authara-org/authara/internal/accesspolicy"

func newAccessPolicy(app *App) accesspolicy.EmailAccessPolicy {
	if !app.Config.AccessPolicy.AllowedEmailEnabled {
		return accesspolicy.NoopEmailAccessPolicy{}
	}

	return accesspolicy.New(accesspolicy.Config{
		Enabled: true,
		Store:   app.Store,
	})
}
