package store

import (
	"context"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store/model"
	"github.com/google/uuid"
)

func toDomainSession(m model.Session) domain.Session {
	return domain.Session{
		ID:     m.ID,
		UserID: m.UserID,

		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,

		ExpiresAt: m.ExpiresAt,
		RevokedAt: m.RevokedAt,

		UserAgent: m.UserAgent,
	}
}

func toModelSession(d domain.Session) model.Session {
	return model.Session{
		UserID: d.UserID,

		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,

		ExpiresAt: d.ExpiresAt,
		RevokedAt: d.RevokedAt,

		UserAgent: d.UserAgent,
	}
}

func toDomainRefreshToken(m model.RefreshToken) domain.RefreshToken {
	return domain.RefreshToken{
		ID:        m.ID,
		SessionID: m.SessionID,

		TokenHash: m.TokenHash,

		CreatedAt:  m.CreatedAt,
		ExpiresAt:  m.ExpiresAt,
		ConsumedAt: m.ConsumedAt,
	}
}

func toModelRefreshToken(d domain.RefreshToken) model.RefreshToken {
	return model.RefreshToken{
		SessionID: d.SessionID,

		TokenHash: d.TokenHash,

		CreatedAt:  d.CreatedAt,
		ExpiresAt:  d.ExpiresAt,
		ConsumedAt: d.ConsumedAt,
	}
}

const sessionColumns = `
	id,
	created_at,
	updated_at,
	user_id,
	expires_at,
	revoked_at,
	user_agent
`

func scanSession(row rowScanner, m *model.Session) error {
	return row.Scan(
		&m.ID,
		&m.CreatedAt,
		&m.UpdatedAt,
		&m.UserID,
		&m.ExpiresAt,
		&m.RevokedAt,
		&m.UserAgent,
	)
}

const refreshTokenColumns = `
	id,
	created_at,
	session_id,
	token_hash,
	expires_at,
	consumed_at
`

func scanRefreshToken(row rowScanner, m *model.RefreshToken) error {
	return row.Scan(
		&m.ID,
		&m.CreatedAt,
		&m.SessionID,
		&m.TokenHash,
		&m.ExpiresAt,
		&m.ConsumedAt,
	)
}

func (s *Store) CreateSession(ctx context.Context, session domain.Session) (domain.Session, error) {
	m := toModelSession(session)

	if err := scanSession(s.queryRow(ctx, `
		INSERT INTO sessions (user_id, expires_at, revoked_at, user_agent)
		VALUES ($1, $2, $3, $4)
		RETURNING `+sessionColumns,
		m.UserID,
		m.ExpiresAt,
		m.RevokedAt,
		m.UserAgent,
	), &m); err != nil {
		return domain.Session{}, err
	}
	return toDomainSession(m), nil
}

func (s *Store) GetSessionByID(ctx context.Context, sessionID uuid.UUID) (domain.Session, error) {
	var m model.Session

	err := scanSession(s.queryRow(ctx, `SELECT `+sessionColumns+` FROM sessions WHERE id = $1`, sessionID), &m)
	if err != nil {
		return domain.Session{}, mapNoRows(err, ErrSessionNotFound)
	}

	return toDomainSession(m), nil
}

func (s *Store) RevokeSession(ctx context.Context, sessionID uuid.UUID, revokedAt time.Time) error {
	_, err := s.exec(ctx, `UPDATE sessions SET revoked_at = $1 WHERE id = $2`, revokedAt, sessionID)
	return err
}

func (s *Store) RevokeAllSessionsForUser(ctx context.Context, userID uuid.UUID, revokedAt time.Time) error {
	_, err := s.exec(ctx, `UPDATE sessions SET revoked_at = $1 WHERE user_id = $2 AND revoked_at IS NULL`, revokedAt, userID)
	return err
}

func (s *Store) CreateRefreshToken(ctx context.Context, token domain.RefreshToken) error {
	m := toModelRefreshToken(token)

	if m.CreatedAt.IsZero() {
		_, err := s.exec(ctx, `
			INSERT INTO refresh_tokens (session_id, token_hash, expires_at, consumed_at)
			VALUES ($1, $2, $3, $4)
		`, m.SessionID, m.TokenHash, m.ExpiresAt, m.ConsumedAt)
		return err
	}

	_, err := s.exec(ctx, `
		INSERT INTO refresh_tokens (created_at, session_id, token_hash, expires_at, consumed_at)
		VALUES ($1, $2, $3, $4, $5)
	`, m.CreatedAt, m.SessionID, m.TokenHash, m.ExpiresAt, m.ConsumedAt)
	return err
}

func (s *Store) GetRefreshTokenByHash(ctx context.Context, hash string) (domain.RefreshToken, error) {
	var m model.RefreshToken

	err := scanRefreshToken(s.queryRow(ctx, `SELECT `+refreshTokenColumns+` FROM refresh_tokens WHERE token_hash = $1`, hash), &m)
	if err != nil {
		return domain.RefreshToken{}, mapNoRows(err, ErrRefreshTokenNotFound)
	}
	return toDomainRefreshToken(m), nil
}

func (s *Store) ConsumeRefreshToken(ctx context.Context, tokenID uuid.UUID, consumedAt time.Time) error {
	res, err := s.exec(ctx, `UPDATE refresh_tokens SET consumed_at = $1 WHERE id = $2 AND consumed_at IS NULL`, consumedAt, tokenID)
	if err != nil {
		return err
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrRefreshTokenNotFound
	}

	return nil
}

func (s *Store) DeleteRefreshTokensBySession(ctx context.Context, sessionID uuid.UUID) error {
	_, err := s.exec(ctx, `DELETE FROM refresh_tokens WHERE session_id = $1`, sessionID)
	return err
}

func (s *Store) DeleteExpiredRefreshTokens(ctx context.Context, now time.Time) error {
	_, err := s.exec(ctx, `DELETE FROM refresh_tokens WHERE expires_at < $1`, now)
	if err != nil {
		return err
	}

	_, err = s.exec(ctx, `DELETE FROM refresh_tokens WHERE consumed_at IS NOT NULL`)
	return err
}

func (s *Store) DeleteExpiredSessions(ctx context.Context, now time.Time) error {
	_, err := s.exec(ctx, `DELETE FROM sessions WHERE expires_at < $1`, now)
	if err != nil {
		return err
	}

	_, err = s.exec(ctx, `DELETE FROM sessions WHERE revoked_at IS NOT NULL`)
	return err
}

func (s *Store) ListActiveSessionsByUserID(ctx context.Context, userID uuid.UUID, now time.Time) ([]domain.Session, error) {
	rows, err := s.queryRows(ctx, `
		SELECT `+sessionColumns+`
		FROM sessions
		WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > $2
		ORDER BY created_at DESC
	`, userID, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.Session, 0)
	for rows.Next() {
		var row model.Session
		if err := scanSession(rows, &row); err != nil {
			return nil, err
		}
		out = append(out, toDomainSession(row))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func (s *Store) GetActiveSessionByID(ctx context.Context, sessionID uuid.UUID, now time.Time) (domain.Session, error) {
	var m model.Session

	err := scanSession(s.queryRow(ctx, `
		SELECT `+sessionColumns+`
		FROM sessions
		WHERE id = $1 AND revoked_at IS NULL AND expires_at > $2
	`, sessionID, now), &m)
	if err != nil {
		return domain.Session{}, mapNoRows(err, ErrSessionNotFound)
	}

	return toDomainSession(m), nil
}

func (s *Store) RevokeOtherSessionsByUserID(ctx context.Context, userID uuid.UUID, keepSessionID uuid.UUID, revokedAt time.Time) error {
	_, err := s.exec(ctx, `
		UPDATE sessions
		SET revoked_at = $1
		WHERE user_id = $2 AND id <> $3 AND revoked_at IS NULL
	`, revokedAt, userID, keepSessionID)
	return err
}

func (s *Store) DeleteRefreshTokensForOtherSessions(ctx context.Context, userID uuid.UUID, keepSessionID uuid.UUID) error {
	_, err := s.exec(ctx, `
		DELETE FROM refresh_tokens
		WHERE session_id IN (
			SELECT id FROM sessions WHERE user_id = $1 AND id <> $2
		)
	`, userID, keepSessionID)
	return err
}

func (s *Store) DeleteRefreshTokensByUserID(ctx context.Context, userID uuid.UUID) error {
	_, err := s.exec(ctx, `
		DELETE FROM refresh_tokens
		WHERE session_id IN (
			SELECT id FROM sessions WHERE user_id = $1
		)
	`, userID)
	return err
}
