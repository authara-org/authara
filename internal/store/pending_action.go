package store

import (
	"context"
	"errors"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func toDomainPendingSignupAction(m model.PendingSignupAction) domain.PendingSignupAction {
	return domain.PendingSignupAction{
		ID:           *m.ID,
		CreatedAt:    m.CreatedAt,
		ChallengeID:  m.ChallengeID,
		Email:        m.Email,
		Username:     m.Username,
		PasswordHash: m.PasswordHash,
	}
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

	err := s.query(ctx).
		Create(&row).
		Error

	if err != nil {
		return domain.PendingSignupAction{}, err
	}

	return toDomainPendingSignupAction(row), nil
}

func (s *Store) GetPendingSignupActionByChallengeID(ctx context.Context, challengeID uuid.UUID) (domain.PendingSignupAction, error) {
	var row model.PendingSignupAction

	err := s.query(ctx).
		Where("challenge_id = ?", challengeID).
		First(&row).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.PendingSignupAction{}, ErrorPendingSignupActionNotFound
		}
		return domain.PendingSignupAction{}, err
	}

	return toDomainPendingSignupAction(row), nil
}

// ============================
// pending password reset
// ============================

func toDomainPendingPasswordReset(m model.PendingPasswordReset) domain.PendingPasswordReset {
	return domain.PendingPasswordReset{
		ID:           *m.ID,
		CreatedAt:    m.CreatedAt,
		ChallengeID:  m.ChallengeID,
		UserID:       m.UserID,
		PasswordHash: m.PasswordHash,
	}
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

	err := s.query(ctx).
		Create(&row).
		Error
	if err != nil {
		return domain.PendingPasswordReset{}, err
	}

	return toDomainPendingPasswordReset(row), nil
}

func (s *Store) GetPendingPasswordResetByChallengeID(ctx context.Context, challengeID uuid.UUID) (domain.PendingPasswordReset, error) {
	var row model.PendingPasswordReset

	err := s.query(ctx).
		Where("challenge_id = ?", challengeID).
		First(&row).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.PendingPasswordReset{}, ErrorPendingPasswordResetNotFound
		}
		return domain.PendingPasswordReset{}, err
	}

	return toDomainPendingPasswordReset(row), nil
}

func (s *Store) DeletePendingPasswordResetByChallengeID(ctx context.Context, challengeID uuid.UUID) error {
	return s.query(ctx).
		Where("challenge_id = ?", challengeID).
		Delete(&model.PendingPasswordReset{}).
		Error
}

// ============================
// pending email change
// ============================

func toDomainPendingEmailChange(m model.PendingEmailChange) domain.PendingEmailChange {
	return domain.PendingEmailChange{
		ID:          *m.ID,
		CreatedAt:   m.CreatedAt,
		ChallengeID: m.ChallengeID,
		UserID:      m.UserID,
		OldEmail:    m.OldEmail,
		NewEmail:    m.NewEmail,
	}
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

	err := s.query(ctx).
		Create(&row).
		Error
	if err != nil {
		return domain.PendingEmailChange{}, err
	}

	return toDomainPendingEmailChange(row), nil
}

func (s *Store) GetPendingEmailChangeByChallengeID(ctx context.Context, challengeID uuid.UUID) (domain.PendingEmailChange, error) {
	var row model.PendingEmailChange

	err := s.query(ctx).
		Where("challenge_id = ?", challengeID).
		First(&row).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.PendingEmailChange{}, ErrorPendingEmailChangeNotFound
		}
		return domain.PendingEmailChange{}, err
	}

	return toDomainPendingEmailChange(row), nil
}

func (s *Store) DeletePendingEmailChangeByChallengeID(ctx context.Context, challengeID uuid.UUID) error {
	res := s.query(ctx).
		Where("challenge_id = ?", challengeID).
		Delete(&model.PendingEmailChange{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrorPendingEmailChangeNotFound
	}
	return nil
}

func (s *Store) UpdateUserEmail(ctx context.Context, userID uuid.UUID, email string) error {
	res := s.query(ctx).
		Model(&model.User{}).
		Where("id = ?", userID).
		Update("email", email)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}
