package store

import (
	"context"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/identity"
	"github.com/authara-org/authara/internal/store/model"
	"github.com/google/uuid"
)

const emailVerificationTransactionColumns = `
	id,
	created_at,
	user_id,
	original_session_id,
	audience,
	authentication_method,
	return_to,
	expires_at,
	consumed_at,
	challenge_id,
	target_email
`

func scanEmailVerificationTransaction(row rowScanner, m *model.EmailVerificationTransaction) error {
	return row.Scan(
		&m.ID,
		&m.CreatedAt,
		&m.UserID,
		&m.OriginalSessionID,
		&m.Audience,
		&m.AuthenticationMethod,
		&m.ReturnTo,
		&m.ExpiresAt,
		&m.ConsumedAt,
		&m.ChallengeID,
		&m.TargetEmail,
	)
}

func toDomainEmailVerificationTransaction(m model.EmailVerificationTransaction) domain.EmailVerificationTransaction {
	return domain.EmailVerificationTransaction{
		ID:                   m.ID,
		CreatedAt:            m.CreatedAt,
		UserID:               m.UserID,
		OriginalSessionID:    m.OriginalSessionID,
		Audience:             m.Audience,
		AuthenticationMethod: domain.AuthenticationMethod(m.AuthenticationMethod),
		ReturnTo:             m.ReturnTo,
		ExpiresAt:            m.ExpiresAt,
		ConsumedAt:           m.ConsumedAt,
		ChallengeID:          m.ChallengeID,
		TargetEmail:          m.TargetEmail,
	}
}

func (s *Store) UpsertEmailVerificationTransaction(ctx context.Context, in domain.EmailVerificationTransaction) (domain.EmailVerificationTransaction, error) {
	var row model.EmailVerificationTransaction
	err := scanEmailVerificationTransaction(s.queryRow(ctx, `
		INSERT INTO email_verification_transactions (
			user_id, original_session_id, audience, authentication_method, return_to, expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id) DO UPDATE
		SET original_session_id = EXCLUDED.original_session_id,
			audience = EXCLUDED.audience,
			authentication_method = EXCLUDED.authentication_method,
			return_to = EXCLUDED.return_to,
			expires_at = EXCLUDED.expires_at,
			consumed_at = NULL
		RETURNING `+emailVerificationTransactionColumns,
		in.UserID,
		in.OriginalSessionID,
		in.Audience,
		string(in.AuthenticationMethod),
		in.ReturnTo,
		in.ExpiresAt,
	), &row)
	if err != nil {
		return domain.EmailVerificationTransaction{}, err
	}
	return toDomainEmailVerificationTransaction(row), nil
}

func (s *Store) GetEmailVerificationTransactionByID(ctx context.Context, id uuid.UUID) (domain.EmailVerificationTransaction, error) {
	return s.getEmailVerificationTransaction(ctx, `id = $1`, id, false)
}

func (s *Store) GetEmailVerificationTransactionByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.EmailVerificationTransaction, error) {
	return s.getEmailVerificationTransaction(ctx, `id = $1`, id, true)
}

func (s *Store) GetEmailVerificationTransactionByChallengeIDForUpdate(ctx context.Context, challengeID uuid.UUID) (domain.EmailVerificationTransaction, error) {
	return s.getEmailVerificationTransaction(ctx, `challenge_id = $1`, challengeID, true)
}

func (s *Store) GetEmailVerificationTransactionByChallengeID(ctx context.Context, challengeID uuid.UUID) (domain.EmailVerificationTransaction, error) {
	return s.getEmailVerificationTransaction(ctx, `challenge_id = $1`, challengeID, false)
}

func (s *Store) getEmailVerificationTransaction(ctx context.Context, where string, value any, forUpdate bool) (domain.EmailVerificationTransaction, error) {
	var row model.EmailVerificationTransaction
	query := `SELECT ` + emailVerificationTransactionColumns + ` FROM email_verification_transactions WHERE ` + where
	if forUpdate {
		query += ` FOR UPDATE`
	}
	if err := scanEmailVerificationTransaction(s.queryRow(ctx, query, value), &row); err != nil {
		return domain.EmailVerificationTransaction{}, mapNoRows(err, ErrEmailVerificationTransactionNotFound)
	}
	return toDomainEmailVerificationTransaction(row), nil
}

func (s *Store) AttachEmailVerificationChallenge(ctx context.Context, id, challengeID uuid.UUID, targetEmail string) error {
	res, err := s.exec(ctx, `
		UPDATE email_verification_transactions
		SET challenge_id = $1, target_email = $2
		WHERE id = $3 AND consumed_at IS NULL
	`, challengeID, identity.CanonicalEmail(targetEmail), id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrEmailVerificationTransactionNotFound
	}
	return nil
}

func (s *Store) ConsumeEmailVerificationTransaction(ctx context.Context, id uuid.UUID, consumedAt time.Time) error {
	res, err := s.exec(ctx, `
		UPDATE email_verification_transactions
		SET consumed_at = $1
		WHERE id = $2 AND consumed_at IS NULL
	`, consumedAt, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrEmailVerificationTransactionNotFound
	}
	return nil
}

func (s *Store) DeleteExpiredEmailVerificationTransactions(ctx context.Context, now time.Time, batchSize int) (int64, error) {
	result, err := s.exec(ctx, `
		WITH oldest AS (
			SELECT id
			FROM email_verification_transactions
			WHERE expires_at < $1
			ORDER BY expires_at, id
			FOR UPDATE SKIP LOCKED
			LIMIT $2
		)
		DELETE FROM email_verification_transactions AS verification
		USING oldest
		WHERE verification.id = oldest.id
	`, now, batchSize)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
