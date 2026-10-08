package store

import (
	"context"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store/model"
	"github.com/google/uuid"
)

const authenticationChallengeColumns = `
	id,
	created_at,
	user_id,
	session_id,
	expires_at,
	consumed_at,
	authentication_method
`

func scanAuthenticationChallenge(row rowScanner, challenge *model.AuthenticationChallenge) error {
	return row.Scan(
		&challenge.ID,
		&challenge.CreatedAt,
		&challenge.UserID,
		&challenge.SessionID,
		&challenge.ExpiresAt,
		&challenge.ConsumedAt,
		&challenge.AuthenticationMethod,
	)
}

func toDomainAuthenticationChallenge(challenge model.AuthenticationChallenge) domain.AuthenticationChallenge {
	out := domain.AuthenticationChallenge{
		ID:         challenge.ID,
		CreatedAt:  challenge.CreatedAt,
		UserID:     challenge.UserID,
		SessionID:  challenge.SessionID,
		ExpiresAt:  challenge.ExpiresAt,
		ConsumedAt: challenge.ConsumedAt,
	}
	if challenge.AuthenticationMethod != nil {
		out.AuthenticationMethod = domain.AuthenticationMethod(*challenge.AuthenticationMethod)
	}
	return out
}

func (s *Store) ReplaceAuthenticationChallenge(
	ctx context.Context,
	challenge domain.AuthenticationChallenge,
) (domain.AuthenticationChallenge, error) {
	var row model.AuthenticationChallenge
	err := scanAuthenticationChallenge(s.queryRow(ctx, `
		INSERT INTO authentication_challenges (id, created_at, user_id, session_id, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (session_id) DO UPDATE
		SET id = EXCLUDED.id,
		    created_at = EXCLUDED.created_at,
		    user_id = EXCLUDED.user_id,
		    expires_at = EXCLUDED.expires_at,
		    consumed_at = NULL,
		    authentication_method = NULL
		RETURNING `+authenticationChallengeColumns,
		challenge.ID,
		challenge.CreatedAt,
		challenge.UserID,
		challenge.SessionID,
		challenge.ExpiresAt,
	), &row)
	if err != nil {
		return domain.AuthenticationChallenge{}, err
	}
	return toDomainAuthenticationChallenge(row), nil
}

func (s *Store) GetAuthenticationChallengeByID(
	ctx context.Context,
	challengeID uuid.UUID,
) (domain.AuthenticationChallenge, error) {
	return s.getAuthenticationChallengeByID(ctx, challengeID, false)
}

func (s *Store) GetAuthenticationChallengeByIDForUpdate(
	ctx context.Context,
	challengeID uuid.UUID,
) (domain.AuthenticationChallenge, error) {
	return s.getAuthenticationChallengeByID(ctx, challengeID, true)
}

func (s *Store) GetAuthenticationChallengeBySessionIDForUpdate(
	ctx context.Context,
	sessionID uuid.UUID,
) (domain.AuthenticationChallenge, error) {
	var row model.AuthenticationChallenge
	if err := scanAuthenticationChallenge(s.queryRow(ctx, `
		SELECT `+authenticationChallengeColumns+`
		FROM authentication_challenges
		WHERE session_id = $1
		FOR UPDATE
	`, sessionID), &row); err != nil {
		return domain.AuthenticationChallenge{}, mapNoRows(err, ErrAuthenticationChallengeNotFound)
	}
	return toDomainAuthenticationChallenge(row), nil
}

func (s *Store) getAuthenticationChallengeByID(
	ctx context.Context,
	challengeID uuid.UUID,
	forUpdate bool,
) (domain.AuthenticationChallenge, error) {
	query := `SELECT ` + authenticationChallengeColumns + ` FROM authentication_challenges WHERE id = $1`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	var row model.AuthenticationChallenge
	if err := scanAuthenticationChallenge(s.queryRow(ctx, query, challengeID), &row); err != nil {
		return domain.AuthenticationChallenge{}, mapNoRows(err, ErrAuthenticationChallengeNotFound)
	}
	return toDomainAuthenticationChallenge(row), nil
}

func (s *Store) ConsumeAuthenticationChallenge(
	ctx context.Context,
	challengeID uuid.UUID,
	method domain.AuthenticationMethod,
	consumedAt time.Time,
) error {
	result, err := s.exec(ctx, `
		UPDATE authentication_challenges
		SET consumed_at = $1, authentication_method = $2
		WHERE id = $3 AND consumed_at IS NULL AND expires_at > $1
	`, consumedAt, string(method), challengeID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrAuthenticationChallengeNotFound
	}
	return nil
}
