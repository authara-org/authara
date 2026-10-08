package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/google/uuid"
)

func (s *Store) UpsertAppleCredential(ctx context.Context, credential domain.AppleCredential, revocationID uuid.UUID) error {
	_, err := s.exec(ctx, `
		WITH previous AS (
			SELECT auth_provider_id, encryption_key_id, encrypted_refresh_token
			FROM apple_credentials
			WHERE auth_provider_id = $1
			FOR UPDATE
		), archived AS (
			INSERT INTO apple_token_revocations (
				id, auth_provider_id, encryption_context, encryption_key_id, encrypted_refresh_token
			)
			SELECT $4, auth_provider_id, auth_provider_id, encryption_key_id, encrypted_refresh_token
			FROM previous
			RETURNING 1
		)
		INSERT INTO apple_credentials (auth_provider_id, encryption_key_id, encrypted_refresh_token)
		SELECT $1, $2, $3
		FROM (SELECT count(*) FROM archived) archived_count
		ON CONFLICT (auth_provider_id) DO UPDATE
		SET encryption_key_id = EXCLUDED.encryption_key_id,
			encrypted_refresh_token = EXCLUDED.encrypted_refresh_token
	`, credential.AuthProviderID, credential.EncryptionKeyID, credential.EncryptedRefreshToken, revocationID)
	return err
}

func (s *Store) GetAppleCredentialByUserID(ctx context.Context, userID uuid.UUID) (domain.AppleCredential, error) {
	var credential domain.AppleCredential
	err := s.queryRow(ctx, `
		SELECT apple.auth_provider_id, apple.encryption_key_id, apple.encrypted_refresh_token,
			apple.created_at, apple.updated_at
		FROM apple_credentials apple
		JOIN auth_providers provider ON provider.id = apple.auth_provider_id
		WHERE provider.user_id = $1 AND provider.provider = $2
	`, userID, string(domain.ProviderApple)).Scan(
		&credential.AuthProviderID,
		&credential.EncryptionKeyID,
		&credential.EncryptedRefreshToken,
		&credential.CreatedAt,
		&credential.UpdatedAt,
	)
	if err != nil {
		return domain.AppleCredential{}, mapNoRows(err, ErrorAuthProviderNotFound)
	}
	return credential, nil
}

func (s *Store) GetAppleCredentialByAuthProviderID(ctx context.Context, authProviderID uuid.UUID) (domain.AppleCredential, error) {
	var credential domain.AppleCredential
	err := s.queryRow(ctx, `
		SELECT auth_provider_id, encryption_key_id, encrypted_refresh_token, created_at, updated_at
		FROM apple_credentials
		WHERE auth_provider_id = $1
	`, authProviderID).Scan(
		&credential.AuthProviderID,
		&credential.EncryptionKeyID,
		&credential.EncryptedRefreshToken,
		&credential.CreatedAt,
		&credential.UpdatedAt,
	)
	if err != nil {
		return domain.AppleCredential{}, mapNoRows(err, ErrorAuthProviderNotFound)
	}
	return credential, nil
}

func (s *Store) CreateAppleTokenRevocation(ctx context.Context, revocation domain.AppleTokenRevocation) error {
	if !revocation.NextAttemptAt.IsZero() {
		_, err := s.exec(ctx, `
			INSERT INTO apple_token_revocations (
				id, auth_provider_id, encryption_context, encryption_key_id,
				encrypted_refresh_token, next_attempt_at
			)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, revocation.ID, revocation.AuthProviderID, revocation.EncryptionContext,
			revocation.EncryptionKeyID, revocation.EncryptedRefreshToken, revocation.NextAttemptAt)
		return err
	}
	_, err := s.exec(ctx, `
		INSERT INTO apple_token_revocations (
			id, encryption_context, encryption_key_id, encrypted_refresh_token
		)
		VALUES ($1, $2, $3, $4)
	`, revocation.ID, revocation.EncryptionContext, revocation.EncryptionKeyID, revocation.EncryptedRefreshToken)
	return err
}

func (s *Store) GetAppleTokenRevocationByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.AppleTokenRevocation, error) {
	var revocation domain.AppleTokenRevocation
	err := s.queryRow(ctx, `
		SELECT id, auth_provider_id, encryption_context, encryption_key_id, encrypted_refresh_token,
			attempt_count, next_attempt_at, last_error, created_at, updated_at
		FROM apple_token_revocations
		WHERE id = $1
		FOR UPDATE
	`, id).Scan(
		&revocation.ID,
		&revocation.AuthProviderID,
		&revocation.EncryptionContext,
		&revocation.EncryptionKeyID,
		&revocation.EncryptedRefreshToken,
		&revocation.AttemptCount,
		&revocation.NextAttemptAt,
		&revocation.LastError,
		&revocation.CreatedAt,
		&revocation.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.AppleTokenRevocation{}, ErrorAppleTokenRevocationNotFound
	}
	return revocation, err
}

func (s *Store) CreateAppleTokenRevocationForUser(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	result, err := s.exec(ctx, `
		INSERT INTO apple_token_revocations (
			id, auth_provider_id, encryption_context, encryption_key_id, encrypted_refresh_token
		)
		SELECT $1, credential.auth_provider_id, credential.auth_provider_id,
			credential.encryption_key_id, credential.encrypted_refresh_token
		FROM apple_credentials credential
		JOIN auth_providers provider ON provider.id = credential.auth_provider_id
		WHERE provider.user_id = $2 AND provider.provider = $3
	`, id, userID, string(domain.ProviderApple))
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return ErrorAuthProviderNotFound
	}
	return nil
}

func (s *Store) GetNextAppleTokenRevocation(ctx context.Context, now time.Time) (domain.AppleTokenRevocation, error) {
	var revocation domain.AppleTokenRevocation
	err := s.queryRow(ctx, `
		SELECT id, auth_provider_id, encryption_context, encryption_key_id, encrypted_refresh_token,
			attempt_count, next_attempt_at, last_error, created_at, updated_at
		FROM apple_token_revocations
		WHERE next_attempt_at <= $1
		ORDER BY next_attempt_at, created_at, id
		FOR UPDATE SKIP LOCKED
		LIMIT 1
	`, now).Scan(
		&revocation.ID,
		&revocation.AuthProviderID,
		&revocation.EncryptionContext,
		&revocation.EncryptionKeyID,
		&revocation.EncryptedRefreshToken,
		&revocation.AttemptCount,
		&revocation.NextAttemptAt,
		&revocation.LastError,
		&revocation.CreatedAt,
		&revocation.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.AppleTokenRevocation{}, ErrorAppleTokenRevocationNotFound
	}
	return revocation, err
}

func (s *Store) RescheduleAppleTokenRevocation(ctx context.Context, id uuid.UUID, nextAttemptAt time.Time, lastError string) error {
	result, err := s.exec(ctx, `
		UPDATE apple_token_revocations
		SET attempt_count = attempt_count + 1,
			next_attempt_at = $2,
			last_error = $3
		WHERE id = $1
	`, id, nextAttemptAt, lastError)
	return ensureOneRow(result, err, ErrorAppleTokenRevocationNotFound)
}

func (s *Store) DeleteAppleTokenRevocation(ctx context.Context, id uuid.UUID) error {
	result, err := s.exec(ctx, `DELETE FROM apple_token_revocations WHERE id = $1`, id)
	return ensureOneRow(result, err, ErrorAppleTokenRevocationNotFound)
}

func ensureOneRow(result sql.Result, err error, notFound error) error {
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return notFound
	}
	return nil
}
