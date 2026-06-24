package bootstrap

import (
	"context"
	"fmt"
	"time"

	"github.com/authara-org/authara/internal/store"
)

func CheckSchemaVersion(store *store.Store, required int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	current, err := store.CurrentSchemaVersion(ctx)
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	if current != required {
		return fmt.Errorf(
			"schema version mismatch: current=%d required=%d",
			current,
			required,
		)
	}

	return nil
}

func EnsureOrganizationMode(store *store.Store, mode string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := store.EnsureOrganizationMode(ctx, mode); err != nil {
		return fmt.Errorf("organization mode check: %w", err)
	}
	return nil
}
