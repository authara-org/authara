package apple

import (
	"context"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store"
	"github.com/authara-org/authara/internal/testutil"
	"github.com/google/uuid"
)

type recordingRevoker struct {
	tokens []string
	err    error
}

func (r *recordingRevoker) Revoke(_ context.Context, token string) error {
	r.tokens = append(r.tokens, token)
	return r.err
}

func TestCredentialsStoreEncryptedRefreshTokenAndSupportKeyRotation(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user, err := tdb.Store.CreateUser(ctx, domain.User{Email: "apple-credential@example.com", Username: "apple-credential"})
		if err != nil {
			t.Fatal(err)
		}
		subject := "apple-credential-subject"
		provider, err := tdb.Store.CreateAuthProvider(ctx, domain.AuthProvider{
			UserID: user.ID, Provider: domain.ProviderApple, ProviderUserID: &subject,
		})
		if err != nil {
			t.Fatal(err)
		}

		key1 := randomCredentialKey(t)
		key2 := randomCredentialKey(t)
		credentials, err := NewCredentials(tdb.Store, "key-1", map[string][]byte{"key-1": key1}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := credentials.Save(ctx, user.ID, "refresh-token-1"); err != nil {
			t.Fatal(err)
		}
		stored, err := tdb.Store.GetAppleCredentialByUserID(ctx, user.ID)
		if err != nil {
			t.Fatal(err)
		}
		if stored.AuthProviderID != provider.ID || stored.EncryptionKeyID != "key-1" || string(stored.EncryptedRefreshToken) == "refresh-token-1" {
			t.Fatalf("unexpected stored credential: %+v", stored)
		}
		if token, err := decrypt(key1, stored.EncryptedRefreshToken, []byte(stored.AuthProviderID.String())); err != nil || string(token) != "refresh-token-1" {
			t.Fatalf("decrypted token = %q, %v", token, err)
		}

		revoker := &recordingRevoker{}
		rotated, err := NewCredentials(tdb.Store, "key-2", map[string][]byte{"key-1": key1, "key-2": key2}, revoker)
		if err != nil {
			t.Fatal(err)
		}
		if token, err := decrypt(key1, stored.EncryptedRefreshToken, []byte(stored.AuthProviderID.String())); err != nil || string(token) != "refresh-token-1" {
			t.Fatalf("read with rotated keyring = %q, %v", token, err)
		}
		if err := rotated.Save(ctx, user.ID, "refresh-token-2"); err != nil {
			t.Fatal(err)
		}
		stored, err = tdb.Store.GetAppleCredentialByUserID(ctx, user.ID)
		if err != nil {
			t.Fatal(err)
		}
		if stored.EncryptionKeyID != "key-2" {
			t.Fatalf("encryption key id = %q", stored.EncryptionKeyID)
		}
		if err := rotated.Save(ctx, user.ID, "refresh-token-2"); err != nil {
			t.Fatal(err)
		}
		processAt := time.Now().UTC().Add(time.Minute)
		processed, more, err := rotated.ProcessRevocationBatch(ctx, processAt)
		if err != nil || processed != 1 || !more {
			t.Fatalf("ProcessRevocationBatch = %d, %v, %v", processed, more, err)
		}
		if processed, _, err := rotated.ProcessRevocationBatch(ctx, processAt); err != nil || processed != 1 {
			t.Fatalf("duplicate ProcessRevocationBatch = %d, %v", processed, err)
		}
		if len(revoker.tokens) != 1 || revoker.tokens[0] != "refresh-token-1" {
			t.Fatalf("revoked tokens = %#v; current token must not be revoked", revoker.tokens)
		}
	})
}

func TestCredentialsRetainFailedRevocationForRetry(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		key := randomCredentialKey(t)
		revoker := &recordingRevoker{err: errors.New("apple unavailable")}
		credentials, err := NewCredentials(tdb.Store, "key-1", map[string][]byte{"key-1": key}, revoker)
		if err != nil {
			t.Fatal(err)
		}
		if err := credentials.QueueRevocation(ctx, "unused-refresh-token"); err != nil {
			t.Fatal(err)
		}
		now := time.Now().UTC()
		if processed, _, err := credentials.ProcessRevocationBatch(ctx, now); err != nil || processed != 1 {
			t.Fatalf("ProcessRevocationBatch = %d, %v", processed, err)
		}
		if len(revoker.tokens) != 1 || revoker.tokens[0] != "unused-refresh-token" {
			t.Fatalf("revoked tokens = %#v", revoker.tokens)
		}
		if _, err := tdb.Store.GetNextAppleTokenRevocation(ctx, now); !errors.Is(err, store.ErrorAppleTokenRevocationNotFound) {
			t.Fatalf("revocation retried too soon: %v", err)
		}
		pending, err := tdb.Store.GetNextAppleTokenRevocation(ctx, now.Add(2*time.Minute))
		if err != nil || pending.AttemptCount != 1 || pending.LastError == nil {
			t.Fatalf("pending revocation = %+v, %v", pending, err)
		}
	})
}

func TestStagedProviderLinkCredentialIsRevokedAfterExpiry(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		revoker := &recordingRevoker{}
		credentials, err := NewCredentials(
			tdb.Store,
			"key-1",
			map[string][]byte{"key-1": randomCredentialKey(t)},
			revoker,
		)
		if err != nil {
			t.Fatal(err)
		}

		now := time.Now().UTC()
		linkID := uuid.New()
		if err := credentials.StageProviderLink(ctx, linkID, "abandoned-refresh-token", now.Add(10*time.Minute)); err != nil {
			t.Fatal(err)
		}
		if processed, more, err := credentials.ProcessRevocationBatch(ctx, now); err != nil || processed != 0 || more {
			t.Fatalf("staged token processed before expiry: %d, %v, %v", processed, more, err)
		}
		if processed, _, err := credentials.ProcessRevocationBatch(ctx, now.Add(11*time.Minute)); err != nil || processed != 1 {
			t.Fatalf("expired staged token processing = %d, %v", processed, err)
		}
		if len(revoker.tokens) != 1 || revoker.tokens[0] != "abandoned-refresh-token" {
			t.Fatalf("revoked tokens = %#v", revoker.tokens)
		}
	})
}

func TestQueuedUserRevocationSurvivesProviderDeletion(t *testing.T) {
	tdb := testutil.OpenTestDB(t)
	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		user, err := tdb.Store.CreateUser(ctx, domain.User{Email: "apple-unlink-queue@example.com", Username: "apple-unlink-queue"})
		if err != nil {
			t.Fatal(err)
		}
		subject := "apple-unlink-queue-subject"
		if _, err := tdb.Store.CreateAuthProvider(ctx, domain.AuthProvider{
			UserID: user.ID, Provider: domain.ProviderApple, ProviderUserID: &subject,
		}); err != nil {
			t.Fatal(err)
		}
		revoker := &recordingRevoker{}
		credentials, err := NewCredentials(
			tdb.Store, "key-1", map[string][]byte{"key-1": randomCredentialKey(t)}, revoker,
		)
		if err != nil {
			t.Fatal(err)
		}
		if err := credentials.Save(ctx, user.ID, "refresh-token-before-unlink"); err != nil {
			t.Fatal(err)
		}
		if err := tdb.Store.CreateAppleTokenRevocationForUser(ctx, uuid.New(), user.ID); err != nil {
			t.Fatal(err)
		}
		if err := tdb.Store.DeleteAuthProviderByMethodAndUserID(ctx, domain.ProviderApple, user.ID); err != nil {
			t.Fatal(err)
		}
		if processed, _, err := credentials.ProcessRevocationBatch(ctx, time.Now().UTC().Add(time.Minute)); err != nil || processed != 1 {
			t.Fatalf("ProcessRevocationBatch = %d, %v", processed, err)
		}
		if len(revoker.tokens) != 1 || revoker.tokens[0] != "refresh-token-before-unlink" {
			t.Fatalf("revoked tokens = %#v", revoker.tokens)
		}
	})
}

func randomCredentialKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	return key
}
