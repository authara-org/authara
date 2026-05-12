package store

import (
	"context"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store/model"
	"github.com/google/uuid"
)

func toDomainPendingSignupAction(m model.PendingSignupAction) domain.PendingSignupAction {
	return domain.PendingSignupAction{
		ID:           m.ID,
		CreatedAt:    m.CreatedAt,
		ChallengeID:  m.ChallengeID,
		Email:        m.Email,
		Username:     m.Username,
		PasswordHash: m.PasswordHash,
	}
}

const pendingSignupActionColumns = `
	id,
	created_at,
	challenge_id,
	email,
	username,
	password_hash
`

func scanPendingSignupAction(row rowScanner, m *model.PendingSignupAction) error {
	return row.Scan(
		&m.ID,
		&m.CreatedAt,
		&m.ChallengeID,
		&m.Email,
		&m.Username,
		&m.PasswordHash,
	)
}

func toModelPendingSignupAction(d domain.PendingSignupAction) model.PendingSignupAction {
	return model.PendingSignupAction{
		ChallengeID:  d.ChallengeID,
		Email:        d.Email,
		Username:     d.Username,
		PasswordHash: d.PasswordHash,
	}
}

func (s *Store) CreatePendingSignupAction(ctx context.Context, in domain.PendingSignupAction) (domain.PendingSignupAction, error) {
	row := toModelPendingSignupAction(in)

	if err := scanPendingSignupAction(s.queryRow(ctx, `
		INSERT INTO pending_signup_actions (challenge_id, email, username, password_hash)
		VALUES ($1, $2, $3, $4)
		RETURNING `+pendingSignupActionColumns,
		row.ChallengeID,
		row.Email,
		row.Username,
		row.PasswordHash,
	), &row); err != nil {
		return domain.PendingSignupAction{}, err
	}

	return toDomainPendingSignupAction(row), nil
}

func (s *Store) GetPendingSignupActionByChallengeID(ctx context.Context, challengeID uuid.UUID) (domain.PendingSignupAction, error) {
	var row model.PendingSignupAction

	err := scanPendingSignupAction(s.queryRow(ctx, `
		SELECT `+pendingSignupActionColumns+`
		FROM pending_signup_actions
		WHERE challenge_id = $1
	`, challengeID), &row)

	if err != nil {
		return domain.PendingSignupAction{}, mapNoRows(err, ErrorPendingSignupActionNotFound)
	}

	return toDomainPendingSignupAction(row), nil
}

// ============================
// pending password reset
// ============================

func toDomainPendingPasswordReset(m model.PendingPasswordReset) domain.PendingPasswordReset {
	return domain.PendingPasswordReset{
		ID:           m.ID,
		CreatedAt:    m.CreatedAt,
		ChallengeID:  m.ChallengeID,
		UserID:       m.UserID,
		PasswordHash: m.PasswordHash,
	}
}

const pendingPasswordResetColumns = `
	id,
	created_at,
	challenge_id,
	user_id,
	password_hash
`

func scanPendingPasswordReset(row rowScanner, m *model.PendingPasswordReset) error {
	return row.Scan(
		&m.ID,
		&m.CreatedAt,
		&m.ChallengeID,
		&m.UserID,
		&m.PasswordHash,
	)
}

func toModelPendingPasswordReset(d domain.PendingPasswordReset) model.PendingPasswordReset {
	return model.PendingPasswordReset{
		ChallengeID:  d.ChallengeID,
		UserID:       d.UserID,
		PasswordHash: d.PasswordHash,
	}
}

func (s *Store) CreatePendingPasswordReset(ctx context.Context, in domain.PendingPasswordReset) (domain.PendingPasswordReset, error) {
	row := toModelPendingPasswordReset(in)

	if err := scanPendingPasswordReset(s.queryRow(ctx, `
		INSERT INTO pending_password_resets (challenge_id, user_id, password_hash)
		VALUES ($1, $2, $3)
		RETURNING `+pendingPasswordResetColumns,
		row.ChallengeID,
		row.UserID,
		row.PasswordHash,
	), &row); err != nil {
		return domain.PendingPasswordReset{}, err
	}

	return toDomainPendingPasswordReset(row), nil
}

func (s *Store) GetPendingPasswordResetByChallengeID(ctx context.Context, challengeID uuid.UUID) (domain.PendingPasswordReset, error) {
	var row model.PendingPasswordReset

	err := scanPendingPasswordReset(s.queryRow(ctx, `
		SELECT `+pendingPasswordResetColumns+`
		FROM pending_password_resets
		WHERE challenge_id = $1
	`, challengeID), &row)
	if err != nil {
		return domain.PendingPasswordReset{}, mapNoRows(err, ErrorPendingPasswordResetNotFound)
	}

	return toDomainPendingPasswordReset(row), nil
}

func (s *Store) DeletePendingPasswordResetByChallengeID(ctx context.Context, challengeID uuid.UUID) error {
	_, err := s.exec(ctx, `DELETE FROM pending_password_resets WHERE challenge_id = $1`, challengeID)
	return err
}

// ============================
// pending email change
// ============================

func toDomainPendingEmailChange(m model.PendingEmailChange) domain.PendingEmailChange {
	return domain.PendingEmailChange{
		ID:          m.ID,
		CreatedAt:   m.CreatedAt,
		ChallengeID: m.ChallengeID,
		UserID:      m.UserID,
		OldEmail:    m.OldEmail,
		NewEmail:    m.NewEmail,
	}
}

const pendingEmailChangeColumns = `
	id,
	created_at,
	challenge_id,
	user_id,
	old_email,
	new_email
`

func scanPendingEmailChange(row rowScanner, m *model.PendingEmailChange) error {
	return row.Scan(
		&m.ID,
		&m.CreatedAt,
		&m.ChallengeID,
		&m.UserID,
		&m.OldEmail,
		&m.NewEmail,
	)
}

func toModelPendingEmailChange(d domain.PendingEmailChange) model.PendingEmailChange {
	return model.PendingEmailChange{
		ChallengeID: d.ChallengeID,
		UserID:      d.UserID,
		OldEmail:    d.OldEmail,
		NewEmail:    d.NewEmail,
	}
}

func (s *Store) CreatePendingEmailChange(ctx context.Context, in domain.PendingEmailChange) (domain.PendingEmailChange, error) {
	row := toModelPendingEmailChange(in)

	if err := scanPendingEmailChange(s.queryRow(ctx, `
		INSERT INTO pending_email_changes (challenge_id, user_id, old_email, new_email)
		VALUES ($1, $2, $3, $4)
		RETURNING `+pendingEmailChangeColumns,
		row.ChallengeID,
		row.UserID,
		row.OldEmail,
		row.NewEmail,
	), &row); err != nil {
		return domain.PendingEmailChange{}, err
	}

	return toDomainPendingEmailChange(row), nil
}

func (s *Store) GetPendingEmailChangeByChallengeID(ctx context.Context, challengeID uuid.UUID) (domain.PendingEmailChange, error) {
	var row model.PendingEmailChange

	err := scanPendingEmailChange(s.queryRow(ctx, `
		SELECT `+pendingEmailChangeColumns+`
		FROM pending_email_changes
		WHERE challenge_id = $1
	`, challengeID), &row)
	if err != nil {
		return domain.PendingEmailChange{}, mapNoRows(err, ErrorPendingEmailChangeNotFound)
	}

	return toDomainPendingEmailChange(row), nil
}

func (s *Store) DeletePendingEmailChangeByChallengeID(ctx context.Context, challengeID uuid.UUID) error {
	res, err := s.exec(ctx, `DELETE FROM pending_email_changes WHERE challenge_id = $1`, challengeID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrorPendingEmailChangeNotFound
	}
	return nil
}

func (s *Store) UpdateUserEmail(ctx context.Context, userID uuid.UUID, email string) error {
	res, err := s.exec(ctx, `UPDATE users SET email = $1 WHERE id = $2`, email, userID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrUserNotFound
	}
	return nil
}
