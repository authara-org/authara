package store

import (
	"context"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store/model"
	"github.com/google/uuid"
)

func toDomainChallenge(m model.Challenge) domain.Challenge {
	return domain.Challenge{
		ID:           m.ID,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
		ExpiresAt:    m.ExpiresAt,
		ConsumedAt:   m.ConsumedAt,
		Purpose:      domain.ChallengePurpose(m.Purpose),
		Email:        m.Email,
		AttemptCount: m.AttemptCount,
		MaxAttempts:  m.MaxAttempts,
		ResendCount:  m.ResendCount,
		MaxResends:   m.MaxResends,
		LastSentAt:   m.LastSentAt,
	}
}

func toModelChallenge(d domain.Challenge) model.Challenge {
	return model.Challenge{
		Purpose:      string(d.Purpose),
		Email:        d.Email,
		ExpiresAt:    d.ExpiresAt,
		ConsumedAt:   d.ConsumedAt,
		AttemptCount: d.AttemptCount,
		MaxAttempts:  d.MaxAttempts,
		ResendCount:  d.ResendCount,
		MaxResends:   d.MaxResends,
		LastSentAt:   d.LastSentAt,
	}
}

func toDomainVerificationCode(m model.VerificationCode) domain.VerificationCode {
	return domain.VerificationCode{
		ID:          m.ID,
		ChallengeID: m.ChallengeID,
		CreatedAt:   m.CreatedAt,
		ExpiresAt:   m.ExpiresAt,
		CodeHash:    m.CodeHash,
	}
}

func toModelVerificationCode(d domain.VerificationCode) model.VerificationCode {
	return model.VerificationCode{
		ChallengeID: d.ChallengeID,
		CreatedAt:   d.CreatedAt,
		ExpiresAt:   d.ExpiresAt,
		CodeHash:    d.CodeHash,
	}
}

const challengeColumns = `
	id,
	created_at,
	updated_at,
	purpose,
	email,
	expires_at,
	consumed_at,
	attempt_count,
	max_attempts,
	resend_count,
	max_resends,
	last_sent_at
`

func scanChallenge(row rowScanner, m *model.Challenge) error {
	return row.Scan(
		&m.ID,
		&m.CreatedAt,
		&m.UpdatedAt,
		&m.Purpose,
		&m.Email,
		&m.ExpiresAt,
		&m.ConsumedAt,
		&m.AttemptCount,
		&m.MaxAttempts,
		&m.ResendCount,
		&m.MaxResends,
		&m.LastSentAt,
	)
}

const verificationCodeColumns = `
	id,
	created_at,
	challenge_id,
	code_hash,
	expires_at
`

func scanVerificationCode(row rowScanner, m *model.VerificationCode) error {
	return row.Scan(
		&m.ID,
		&m.CreatedAt,
		&m.ChallengeID,
		&m.CodeHash,
		&m.ExpiresAt,
	)
}

func (s *Store) CreateChallenge(ctx context.Context, in domain.Challenge) (domain.Challenge, error) {
	row := toModelChallenge(in)

	if err := scanChallenge(s.queryRow(ctx, `
		INSERT INTO challenges (
			purpose,
			email,
			expires_at,
			consumed_at,
			attempt_count,
			max_attempts,
			resend_count,
			max_resends,
			last_sent_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING `+challengeColumns,
		row.Purpose,
		row.Email,
		row.ExpiresAt,
		row.ConsumedAt,
		row.AttemptCount,
		row.MaxAttempts,
		row.ResendCount,
		row.MaxResends,
		row.LastSentAt,
	), &row); err != nil {
		return domain.Challenge{}, err
	}

	return toDomainChallenge(row), nil
}

func (s *Store) GetChallengeByID(ctx context.Context, challengeID uuid.UUID) (domain.Challenge, error) {
	return s.getChallengeByID(ctx, challengeID, false)
}

func (s *Store) GetChallengeByIDForUpdate(ctx context.Context, challengeID uuid.UUID) (domain.Challenge, error) {
	return s.getChallengeByID(ctx, challengeID, true)
}

func (s *Store) getChallengeByID(ctx context.Context, challengeID uuid.UUID, forUpdate bool) (domain.Challenge, error) {
	var row model.Challenge

	query := `SELECT ` + challengeColumns + ` FROM challenges WHERE id = $1`
	if forUpdate {
		query += ` FOR UPDATE`
	}

	err := scanChallenge(s.queryRow(ctx, query, challengeID), &row)
	if err != nil {
		return domain.Challenge{}, mapNoRows(err, ErrorChallengeNotFound)
	}

	return toDomainChallenge(row), nil
}

func (s *Store) ConsumeChallenge(ctx context.Context, challengeID uuid.UUID, now time.Time) error {
	res, err := s.exec(ctx, `
		UPDATE challenges
		SET consumed_at = $1
		WHERE id = $2 AND consumed_at IS NULL
	`, now, challengeID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrorChallengeAlreadyConsumed
	}
	return nil
}

func (s *Store) IncrementChallengeAttemptCount(ctx context.Context, challengeID uuid.UUID) error {
	_, err := s.exec(ctx, `
		UPDATE challenges
		SET attempt_count = attempt_count + 1
		WHERE id = $1
	`, challengeID)
	return err
}

func (s *Store) IncrementChallengeResendCount(ctx context.Context, challengeID uuid.UUID, now time.Time) (bool, error) {
	result, err := s.exec(ctx, `
		UPDATE challenges
		SET resend_count = resend_count + 1,
		    last_sent_at = $1
		WHERE id = $2 AND resend_count < max_resends
	`, now, challengeID)
	if err != nil {
		return false, err
	}

	// RowsAffected == 0 means limit reached (or not found)
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected == 1, nil
}

func (s *Store) SetChallengeLastSentAt(ctx context.Context, challengeID uuid.UUID, now time.Time) error {
	_, err := s.exec(ctx, `UPDATE challenges SET last_sent_at = $1 WHERE id = $2`, now, challengeID)
	return err
}

func (s *Store) DeleteSentEmailJobsBefore(ctx context.Context, t time.Time) error {
	_, err := s.exec(ctx, `DELETE FROM email_jobs WHERE status = $1 AND sent_at < $2`, string(domain.EmailJobStatusSent), t)
	return err
}

func (s *Store) DeleteFailedEmailJobsBefore(ctx context.Context, t time.Time) error {
	_, err := s.exec(ctx, `DELETE FROM email_jobs WHERE status = $1 AND created_at < $2`, string(domain.EmailJobStatusFailed), t)
	return err
}

func (s *Store) DeleteExpiredChallenges(ctx context.Context, now time.Time) error {
	_, err := s.exec(ctx, `DELETE FROM challenges WHERE expires_at < $1`, now)
	return err
}

func (s *Store) UpsertVerificationCode(ctx context.Context, in domain.VerificationCode) error {
	row := toModelVerificationCode(in)

	_, err := s.exec(ctx, `
		INSERT INTO verification_codes (challenge_id, code_hash, expires_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (challenge_id) DO UPDATE
		SET code_hash = EXCLUDED.code_hash,
		    expires_at = EXCLUDED.expires_at,
		    created_at = now()
	`, row.ChallengeID, row.CodeHash, row.ExpiresAt)
	return err
}

func (s *Store) GetVerificationCodeByChallengeID(ctx context.Context, challengeID uuid.UUID) (domain.VerificationCode, error) {
	var row model.VerificationCode

	err := scanVerificationCode(s.queryRow(ctx, `
		SELECT `+verificationCodeColumns+`
		FROM verification_codes
		WHERE challenge_id = $1
	`, challengeID), &row)
	if err != nil {
		return domain.VerificationCode{}, mapNoRows(err, ErrorVerificationCodeNotFound)
	}

	return toDomainVerificationCode(row), nil
}
