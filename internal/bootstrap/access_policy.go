package bootstrap

import (
	"context"
	"fmt"
	"time"

	"github.com/authara-org/authara/internal/accesspolicy"
)

type bootstrapAllowedEmailStore interface {
	EnsureAllowedEmail(context.Context, string) error
}

func EnsureBootstrapAllowedEmails(store bootstrapAllowedEmailStore, emails []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for i, email := range emails {
		if err := store.EnsureAllowedEmail(ctx, email); err != nil {
			return fmt.Errorf("ensure bootstrap allowed email %d: %w", i+1, err)
		}
	}
	return nil
}

func newAccessPolicy(app *App) accesspolicy.EmailAccessPolicy {
	return accesspolicy.New(accesspolicy.Config{
		Store:  app.Store,
		Policy: app.Config,
	})
}
