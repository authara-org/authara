package bootstrap

import "github.com/authara-org/authara/internal/accesspolicy"

func newAccessPolicy(app *App) accesspolicy.EmailAccessPolicy {
	return accesspolicy.New(accesspolicy.Config{
		Store:  app.Store,
		Policy: app.Config,
	})
}
