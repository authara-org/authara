package auth

import (
	"context"
	"testing"

	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/store/tx"
)

func setupAuthService(t *testing.T) (*Service, context.Context) {
	t.Helper()

	// In real tests:
	// - this connects to a real Postgres started by CI / docker-compose
	// - migrations are already applied
	db, err := store.New(store.Config{
		Host:     "localhost",
		Port:     5432,
		Username: "authara",
		Password: "authara",
		Database: "authara_test",
		Schema:   "public",
		Timezone: "UTC",
		LogSql:   false,
	})
	if err != nil {
		t.Fatalf("failed to connect to test DB: %v", err)
	}

	txManager := tx.New(db)

	authService := New(Config{
		Store: db,
		Tx:    txManager,
	})

	ctx := context.Background()

	return authService, ctx
}

func TestSignup_CreatesUserAndProvider(t *testing.T) {
	// authService, ctx := setupAuthService(t)
	//
	// _, err := authService.Signup(
	// 	ctx,
	// 	"test@example.com",
	// 	"super-secret-password",
	// )
	// if err != nil {
	// 	t.Fatalf("expected signup to succeed, got error: %v", err)
	// }

	// if user == nil {
	// 	t.Fatal("expected user to be returned")
	// }
	//
	// if user.Email != "test@example.com" {
	// 	t.Fatalf("expected email to match, got %s", user.Email)
	// }
}
