package passkey_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/auth"
	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/passkey"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/google/uuid"
)

func TestDeletePasskey_BlocksLastPasskey(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		svc := newTestPasskeyService(t, tdb)
		user := createPasskeyTestUser(t, ctx, tdb, "last-passkey@example.com", "last-passkey")
		p := createPasskeyTestPasskey(t, ctx, tdb, user.ID, "last-passkey-credential")

		err := svc.DeletePasskey(ctx, user.ID, p.ID)
		if !errors.Is(err, auth.ErrCannotRemoveLastAuthMethod) {
			t.Fatalf("expected ErrCannotRemoveLastAuthMethod, got %v", err)
		}

		passkeys, err := tdb.Store.ListPasskeysByUserID(ctx, user.ID)
		if err != nil {
			t.Fatalf("ListPasskeysByUserID failed: %v", err)
		}
		if len(passkeys) != 1 {
			t.Fatalf("expected passkey to remain, got %d passkeys", len(passkeys))
		}
	})
}

func TestDeletePasskey_AllowsOneOfMultiplePasskeys(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		svc := newTestPasskeyService(t, tdb)
		user := createPasskeyTestUser(t, ctx, tdb, "multiple-passkeys@example.com", "multiple-passkeys")
		first := createPasskeyTestPasskey(t, ctx, tdb, user.ID, "multiple-passkeys-1")
		createPasskeyTestPasskey(t, ctx, tdb, user.ID, "multiple-passkeys-2")

		if err := svc.DeletePasskey(ctx, user.ID, first.ID); err != nil {
			t.Fatalf("DeletePasskey failed: %v", err)
		}

		passkeys, err := tdb.Store.ListPasskeysByUserID(ctx, user.ID)
		if err != nil {
			t.Fatalf("ListPasskeysByUserID failed: %v", err)
		}
		if len(passkeys) != 1 {
			t.Fatalf("expected 1 remaining passkey, got %d", len(passkeys))
		}
	})
}

func TestDeletePasskey_AllowsPasskeyWhenPasswordExists(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		svc := newTestPasskeyService(t, tdb)
		user := createPasskeyPasswordUser(t, ctx, tdb, "password-plus-passkey@example.com", "password-plus-passkey")
		p := createPasskeyTestPasskey(t, ctx, tdb, user.ID, "password-plus-passkey-credential")

		if err := svc.DeletePasskey(ctx, user.ID, p.ID); err != nil {
			t.Fatalf("DeletePasskey failed: %v", err)
		}

		passkeys, err := tdb.Store.ListPasskeysByUserID(ctx, user.ID)
		if err != nil {
			t.Fatalf("ListPasskeysByUserID failed: %v", err)
		}
		if len(passkeys) != 0 {
			t.Fatalf("expected passkey to be deleted, got %d passkeys", len(passkeys))
		}
	})
}

func TestDeletePasskeySerializesConcurrentAuthMethodRemoval(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	ctx := context.Background()
	svc := newTestPasskeyService(t, tdb)
	user := createPasskeyTestUser(
		t,
		ctx,
		tdb,
		"concurrent-delete-"+uuid.NewString()+"@example.com",
		"concurrent-delete-"+uuid.NewString(),
	)
	t.Cleanup(func() {
		_ = tdb.Store.DeleteUser(context.Background(), user.ID)
	})

	first := createPasskeyTestPasskey(t, ctx, tdb, user.ID, "concurrent-passkey-1-"+uuid.NewString())
	second := createPasskeyTestPasskey(t, ctx, tdb, user.ID, "concurrent-passkey-2-"+uuid.NewString())

	txCtx, cancel, err := tdb.Tx.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer cancel()

	if err := tdb.Store.LockUserForAuthMethodMutation(txCtx, user.ID); err != nil {
		t.Fatalf("LockUserForAuthMethodMutation failed: %v", err)
	}
	if err := tdb.Store.DeletePasskeyByIDAndUserID(txCtx, first.ID, user.ID); err != nil {
		t.Fatalf("DeletePasskeyByIDAndUserID failed: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- svc.DeletePasskey(ctx, user.ID, second.ID)
	}()

	select {
	case err := <-done:
		t.Fatalf("DeletePasskey completed before auth-method lock was released: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	if err := tdb.Tx.Commit(txCtx); err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	select {
	case err := <-done:
		if !errors.Is(err, auth.ErrCannotRemoveLastAuthMethod) {
			t.Fatalf("expected ErrCannotRemoveLastAuthMethod, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("DeletePasskey did not finish after auth-method lock was released")
	}

	passkeys, err := tdb.Store.ListPasskeysByUserID(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListPasskeysByUserID failed: %v", err)
	}
	if len(passkeys) != 1 || passkeys[0].ID != second.ID {
		t.Fatalf("expected only second passkey to remain, got %+v", passkeys)
	}
}

func newTestPasskeyService(t *testing.T, tdb *testutil.TestDB) *passkey.Service {
	t.Helper()

	svc, err := passkey.New(passkey.Config{
		RPDisplayName: "Authara",
		RPID:          "localhost",
		RPOrigins:     []string{"http://localhost:3000"},
		Store:         tdb.Store,
		Tx:            tdb.Tx,
	})
	if err != nil {
		t.Fatalf("passkey.New failed: %v", err)
	}

	return svc
}

func createPasskeyTestUser(
	t *testing.T,
	ctx context.Context,
	tdb *testutil.TestDB,
	email string,
	username string,
) domain.User {
	t.Helper()

	user, err := tdb.Store.CreateUser(ctx, domain.User{
		Email:    email,
		Username: username,
	})
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	return user
}

func createPasskeyPasswordUser(
	t *testing.T,
	ctx context.Context,
	tdb *testutil.TestDB,
	email string,
	username string,
) domain.User {
	t.Helper()

	user := createPasskeyTestUser(t, ctx, tdb, email, username)
	hash := "hashed-password"
	_, err := tdb.Store.CreateAuthProvider(ctx, domain.AuthProvider{
		UserID:       user.ID,
		Provider:     domain.ProviderPassword,
		PasswordHash: &hash,
	})
	if err != nil {
		t.Fatalf("CreateAuthProvider failed: %v", err)
	}
	return user
}

func createPasskeyTestPasskey(
	t *testing.T,
	ctx context.Context,
	tdb *testutil.TestDB,
	userID uuid.UUID,
	credentialID string,
) domain.Passkey {
	t.Helper()

	p, err := tdb.Store.CreatePasskey(ctx, domain.Passkey{
		UserID:       userID,
		CredentialID: []byte(credentialID),
		PublicKey:    []byte("public-key-" + credentialID),
		Name:         "Passkey",
	})
	if err != nil {
		t.Fatalf("CreatePasskey failed: %v", err)
	}
	return p
}
