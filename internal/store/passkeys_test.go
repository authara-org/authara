package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/google/uuid"
)

func TestPasskeys_CreateListUniqueAndDelete(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user := createStorePasskeyUser(t, ctx, tdb, "store-passkey@example.com", "store-passkey")

		created, err := tdb.Store.CreatePasskey(ctx, domain.Passkey{
			UserID:            user.ID,
			CredentialID:      []byte("credential-1"),
			PublicKey:         []byte("public-key-1"),
			AttestationType:   "none",
			AttestationFormat: "none",
			Transport:         []string{"internal", "hybrid"},
			SignCount:         7,
			Name:              "Laptop",
			UserPresent:       true,
			UserVerified:      true,
			BackupEligible:    true,
			BackupState:       true,
		})
		if err != nil {
			t.Fatalf("CreatePasskey failed: %v", err)
		}
		if created.ID == user.ID {
			t.Fatal("expected passkey id to differ from user id")
		}
		if created.Name != "Laptop" {
			t.Fatalf("expected name Laptop, got %q", created.Name)
		}

		passkeys, err := tdb.Store.ListPasskeysByUserID(ctx, user.ID)
		if err != nil {
			t.Fatalf("ListPasskeysByUserID failed: %v", err)
		}
		if len(passkeys) != 1 {
			t.Fatalf("expected 1 passkey, got %d", len(passkeys))
		}
		if string(passkeys[0].CredentialID) != "credential-1" {
			t.Fatalf("unexpected credential id %q", string(passkeys[0].CredentialID))
		}

		byCredential, err := tdb.Store.GetPasskeyByCredentialID(ctx, []byte("credential-1"))
		if err != nil {
			t.Fatalf("GetPasskeyByCredentialID failed: %v", err)
		}
		if byCredential.ID != created.ID {
			t.Fatalf("expected passkey id %q, got %q", created.ID, byCredential.ID)
		}

		if err := tdb.Store.DeletePasskeyByIDAndUserID(ctx, created.ID, user.ID); err != nil {
			t.Fatalf("DeletePasskeyByIDAndUserID failed: %v", err)
		}

		passkeys, err = tdb.Store.ListPasskeysByUserID(ctx, user.ID)
		if err != nil {
			t.Fatalf("ListPasskeysByUserID failed after delete: %v", err)
		}
		if len(passkeys) != 0 {
			t.Fatalf("expected 0 passkeys after delete, got %d", len(passkeys))
		}
	})
}

func TestCreatePasskey_DuplicateCredentialID(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user := createStorePasskeyUser(t, ctx, tdb, "duplicate-passkey@example.com", "duplicate-passkey")
		createStorePasskey(t, ctx, tdb, user.ID, "duplicate-credential")

		_, err := tdb.Store.CreatePasskey(ctx, domain.Passkey{
			UserID:       user.ID,
			CredentialID: []byte("duplicate-credential"),
			PublicKey:    []byte("public-key-duplicate"),
		})
		if !errors.Is(err, store.ErrPasskeyAlreadyExists) {
			t.Fatalf("expected ErrPasskeyAlreadyExists, got %v", err)
		}
	})
}

func TestCountAuthMethods(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		passwordUser := createStorePasswordUser(t, ctx, tdb, "password-count@example.com", "password-count")
		assertStoreAuthMethodCount(t, ctx, tdb, passwordUser.ID, 1)

		passkeyOnly := createStorePasskeyUser(t, ctx, tdb, "passkey-count@example.com", "passkey-count")
		createStorePasskey(t, ctx, tdb, passkeyOnly.ID, "passkey-only-1")
		assertStoreAuthMethodCount(t, ctx, tdb, passkeyOnly.ID, 1)

		both := createStorePasswordUser(t, ctx, tdb, "both-count@example.com", "both-count")
		createStorePasskey(t, ctx, tdb, both.ID, "both-passkey-1")
		assertStoreAuthMethodCount(t, ctx, tdb, both.ID, 2)

		createStorePasskey(t, ctx, tdb, both.ID, "both-passkey-2")
		assertStoreAuthMethodCount(t, ctx, tdb, both.ID, 3)
	})
}

func createStorePasskeyUser(
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

func createStorePasswordUser(
	t *testing.T,
	ctx context.Context,
	tdb *testutil.TestDB,
	email string,
	username string,
) domain.User {
	t.Helper()

	user := createStorePasskeyUser(t, ctx, tdb, email, username)
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

func createStorePasskey(
	t *testing.T,
	ctx context.Context,
	tdb *testutil.TestDB,
	userID uuid.UUID,
	credentialID string,
) domain.Passkey {
	t.Helper()

	passkey, err := tdb.Store.CreatePasskey(ctx, domain.Passkey{
		UserID:       userID,
		CredentialID: []byte(credentialID),
		PublicKey:    []byte("public-key-" + credentialID),
		Name:         "Passkey",
	})
	if err != nil {
		t.Fatalf("CreatePasskey failed: %v", err)
	}

	return passkey
}

func assertStoreAuthMethodCount(
	t *testing.T,
	ctx context.Context,
	tdb *testutil.TestDB,
	userID uuid.UUID,
	want int,
) {
	t.Helper()

	got, err := tdb.Store.CountAuthMethods(ctx, userID)
	if err != nil {
		t.Fatalf("CountAuthMethods failed: %v", err)
	}
	if got != want {
		t.Fatalf("expected %d auth methods, got %d", want, got)
	}
}
