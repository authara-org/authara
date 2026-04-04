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

	err := s.db.WithContext(ctx).
		Create(&row).
		Error

	if err != nil {
		return domain.PendingSignupAction{}, err
	}

	return toDomainPendingSignupAction(row), nil
}

func (s *Store) GetPendingSignupActionByChallengeID(ctx context.Context, challengeID uuid.UUID) (domain.PendingSignupAction, error) {
	var row model.PendingSignupAction

	err := s.db.WithContext(ctx).
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
