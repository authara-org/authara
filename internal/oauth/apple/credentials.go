package apple

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store"
	"github.com/google/uuid"
)

type Credentials struct {
	store       *store.Store
	activeKeyID string
	keys        map[string][]byte
	revoker     interface {
		Revoke(context.Context, string) error
	}
}

const (
	revocationTimeout   = 5 * time.Second
	revocationRetryBase = time.Minute
	revocationRetryMax  = 24 * time.Hour
)

func NewCredentials(database *store.Store, activeKeyID string, keys map[string][]byte, revoker interface {
	Revoke(context.Context, string) error
}) (*Credentials, error) {
	if database == nil {
		return nil, errors.New("apple credential store is required")
	}
	if activeKeyID == "" {
		return nil, errors.New("apple credential active key id is required")
	}
	if len(keys[activeKeyID]) != 32 {
		return nil, errors.New("apple credential active key must contain 32 bytes")
	}
	cloned := make(map[string][]byte, len(keys))
	for id, key := range keys {
		if len(key) != 32 {
			return nil, fmt.Errorf("apple credential key %q must contain 32 bytes", id)
		}
		cloned[id] = append([]byte(nil), key...)
	}
	return &Credentials{store: database, activeKeyID: activeKeyID, keys: cloned, revoker: revoker}, nil
}

func (c *Credentials) Revoke(ctx context.Context, refreshToken string) error {
	if c.revoker == nil || refreshToken == "" {
		return nil
	}
	return c.revoker.Revoke(ctx, refreshToken)
}

func (c *Credentials) Save(ctx context.Context, userID uuid.UUID, refreshToken string) error {
	if refreshToken == "" {
		return errors.New("apple refresh token is empty")
	}
	provider, err := c.store.GetAuthProviderByMethodAndUserIDForUpdate(ctx, domain.ProviderApple, userID)
	if err != nil {
		return err
	}
	ciphertext, err := encrypt(c.keys[c.activeKeyID], []byte(refreshToken), []byte(provider.ID.String()))
	if err != nil {
		return err
	}
	return c.store.UpsertAppleCredential(ctx, domain.AppleCredential{
		AuthProviderID: provider.ID, EncryptionKeyID: c.activeKeyID, EncryptedRefreshToken: ciphertext,
	}, uuid.New())
}

func (c *Credentials) QueueRevocation(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}
	id := uuid.New()
	ciphertext, err := encrypt(c.keys[c.activeKeyID], []byte(refreshToken), []byte(id.String()))
	if err != nil {
		return err
	}
	return c.store.CreateAppleTokenRevocation(ctx, domain.AppleTokenRevocation{
		ID: id, EncryptionContext: id, EncryptionKeyID: c.activeKeyID, EncryptedRefreshToken: ciphertext,
	})
}

// StageProviderLink keeps a newly issued refresh token encrypted until the
// existing account owner proves control. If the link is abandoned, the normal
// revocation worker processes the token when the pending link expires.
func (c *Credentials) StageProviderLink(ctx context.Context, linkID uuid.UUID, refreshToken string, expiresAt time.Time) error {
	if linkID == uuid.Nil || refreshToken == "" || expiresAt.IsZero() {
		return errors.New("pending Apple provider credential is invalid")
	}
	ciphertext, err := encrypt(c.keys[c.activeKeyID], []byte(refreshToken), []byte(linkID.String()))
	if err != nil {
		return err
	}
	return c.store.CreateAppleTokenRevocation(ctx, domain.AppleTokenRevocation{
		ID:                    linkID,
		EncryptionContext:     linkID,
		EncryptionKeyID:       c.activeKeyID,
		EncryptedRefreshToken: ciphertext,
		NextAttemptAt:         expiresAt,
	})
}

// PromoteProviderLink turns a staged token into the linked account's current
// Apple credential. Callers invoke this inside the provider-link transaction.
func (c *Credentials) PromoteProviderLink(ctx context.Context, userID uuid.UUID, linkID uuid.UUID) error {
	revocation, err := c.store.GetAppleTokenRevocationByIDForUpdate(ctx, linkID)
	if err != nil {
		return err
	}
	if revocation.AuthProviderID != nil || revocation.EncryptionContext != linkID {
		return errors.New("pending Apple provider credential is invalid")
	}
	key, ok := c.keys[revocation.EncryptionKeyID]
	if !ok {
		return fmt.Errorf("apple credential encryption key %q is unavailable", revocation.EncryptionKeyID)
	}
	plaintext, err := decrypt(key, revocation.EncryptedRefreshToken, []byte(linkID.String()))
	if err != nil {
		return err
	}
	if err := c.Save(ctx, userID, string(plaintext)); err != nil {
		return err
	}
	return c.store.DeleteAppleTokenRevocation(ctx, linkID)
}

// ProcessRevocationBatch processes one token at a time. The maintenance
// coordinator supplies the transaction and leader fencing, while failures are
// committed with a later retry time so one unavailable token cannot block the
// rest of the queue.
func (c *Credentials) ProcessRevocationBatch(ctx context.Context, now time.Time) (int64, bool, error) {
	revocation, err := c.store.GetNextAppleTokenRevocation(ctx, now)
	if errors.Is(err, store.ErrorAppleTokenRevocationNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}

	key, ok := c.keys[revocation.EncryptionKeyID]
	if !ok {
		err = fmt.Errorf("apple credential encryption key %q is unavailable", revocation.EncryptionKeyID)
	} else {
		var plaintext []byte
		plaintext, err = decrypt(key, revocation.EncryptedRefreshToken, []byte(revocation.EncryptionContext.String()))
		if err == nil && revocation.AuthProviderID != nil {
			var current domain.AppleCredential
			current, err = c.store.GetAppleCredentialByAuthProviderID(ctx, *revocation.AuthProviderID)
			switch {
			case err == nil:
				currentKey, keyOK := c.keys[current.EncryptionKeyID]
				if !keyOK {
					err = fmt.Errorf("apple credential encryption key %q is unavailable", current.EncryptionKeyID)
					break
				}
				var currentPlaintext []byte
				currentPlaintext, err = decrypt(currentKey, current.EncryptedRefreshToken, []byte(current.AuthProviderID.String()))
				if err == nil && subtle.ConstantTimeCompare(plaintext, currentPlaintext) == 1 {
					return 1, true, c.store.DeleteAppleTokenRevocation(ctx, revocation.ID)
				}
			case errors.Is(err, store.ErrorAuthProviderNotFound):
				err = nil
			}
		}
		if err == nil {
			revokeCtx, cancel := context.WithTimeout(ctx, revocationTimeout)
			err = c.Revoke(revokeCtx, string(plaintext))
			cancel()
		}
	}
	if err != nil {
		nextAttempt := now.Add(revocationRetryDelay(revocation.AttemptCount))
		if updateErr := c.store.RescheduleAppleTokenRevocation(ctx, revocation.ID, nextAttempt, err.Error()); updateErr != nil {
			return 1, true, updateErr
		}
		return 1, true, nil
	}
	if err := c.store.DeleteAppleTokenRevocation(ctx, revocation.ID); err != nil {
		return 1, true, err
	}
	return 1, true, nil
}

func revocationRetryDelay(attempt int) time.Duration {
	shift := attempt
	if shift > 10 {
		shift = 10
	}
	delay := revocationRetryBase * time.Duration(1<<shift)
	if delay > revocationRetryMax {
		return revocationRetryMax
	}
	return delay
}

func encrypt(key, plaintext, additionalData []byte) ([]byte, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return aead.Seal(nonce, nonce, plaintext, additionalData), nil
}

func decrypt(key, ciphertext, additionalData []byte) ([]byte, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < aead.NonceSize() {
		return nil, errors.New("apple credential ciphertext is invalid")
	}
	nonce := ciphertext[:aead.NonceSize()]
	return aead.Open(nil, nonce, ciphertext[aead.NonceSize():], additionalData)
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
