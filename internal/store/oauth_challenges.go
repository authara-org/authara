package store

import (
	"context"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/google/uuid"
)

const oauthAuthenticationChallengeColumns = `
	id,
	created_at,
	provider,
	state,
	nonce,
	expires_at,
	consumed_at
`

func (s *Store) CreateOAuthAuthenticationChallenge(ctx context.Context, challenge domain.OAuthAuthenticationChallenge) (domain.OAuthAuthenticationChallenge, error) {
	var created domain.OAuthAuthenticationChallenge
	var provider string
	err := s.queryRow(ctx, `
		INSERT INTO oauth_authentication_challenges (provider, state, nonce, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING `+oauthAuthenticationChallengeColumns,
		challenge.Provider, challenge.State, challenge.Nonce, challenge.ExpiresAt,
	).Scan(&created.ID, &created.CreatedAt, &provider, &created.State, &created.Nonce, &created.ExpiresAt, &created.ConsumedAt)
	created.Provider = domain.Provider(provider)
	return created, err
}

func (s *Store) GetOAuthAuthenticationChallengeForUpdate(ctx context.Context, id uuid.UUID) (domain.OAuthAuthenticationChallenge, error) {
	var challenge domain.OAuthAuthenticationChallenge
	var provider string
	err := s.queryRow(ctx, `
		SELECT `+oauthAuthenticationChallengeColumns+`
		FROM oauth_authentication_challenges
		WHERE id = $1
		FOR UPDATE
	`, id).Scan(&challenge.ID, &challenge.CreatedAt, &provider, &challenge.State, &challenge.Nonce, &challenge.ExpiresAt, &challenge.ConsumedAt)
	if err != nil {
		return domain.OAuthAuthenticationChallenge{}, mapNoRows(err, ErrOAuthAuthenticationChallengeNotFound)
	}
	challenge.Provider = domain.Provider(provider)
	return challenge, nil
}

func (s *Store) ConsumeOAuthAuthenticationChallenge(ctx context.Context, id uuid.UUID, now time.Time) error {
	result, err := s.exec(ctx, `
		UPDATE oauth_authentication_challenges
		SET consumed_at = $1
		WHERE id = $2 AND consumed_at IS NULL
	`, now, id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrOAuthAuthenticationChallengeConsumed
	}
	return nil
}

func (s *Store) DeleteExpiredOAuthAuthenticationChallenges(ctx context.Context, now time.Time, batchSize int) (int64, error) {
	result, err := s.exec(ctx, `
		WITH oldest AS (
			SELECT id
			FROM oauth_authentication_challenges
			WHERE expires_at < $1 OR consumed_at IS NOT NULL
			ORDER BY LEAST(expires_at, COALESCE(consumed_at, expires_at)), id
			FOR UPDATE SKIP LOCKED
			LIMIT $2
		)
		DELETE FROM oauth_authentication_challenges AS challenge
		USING oldest
		WHERE challenge.id = oldest.id
	`, now, batchSize)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
