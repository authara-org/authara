package store

import (
	"context"
	"errors"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func toDomainChallenge(m model.Challenge) domain.Challenge {
	return domain.Challenge{
		ID:           *m.ID,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
		ExpiresAt:    m.ExpiresAt,
		ConsumedAt:   m.ConsumedAt,
		Purpose:      domain.ChallengePurpose(m.Purpose),
		Email:        m.Email,
		AttemptCount: m.AttemptCount,
		MaxAttempts:  m.MaxAttempts,
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
	}
}

func toDomainVerificationCode(m model.VerificationCode) domain.VerificationCode {
	return domain.VerificationCode{
		ID:          *m.ID,
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

func (s *Store) CreateChallenge(ctx context.Context, in domain.Challenge) (domain.Challenge, error) {
	row := toModelChallenge(in)

	err := s.db.WithContext(ctx).
		Create(&row).
		Error

	if err != nil {
		return domain.Challenge{}, err
	}

	return toDomainChallenge(row), nil
}

func (s *Store) GetChallengeByID(ctx context.Context, challengeID uuid.UUID) (domain.Challenge, error) {
	var row model.Challenge

	err := s.db.WithContext(ctx).
		Where("id = ?", challengeID).
		First(&row).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Challenge{}, ErrorChallengeNotFound
		}
		return domain.Challenge{}, err
	}

	return toDomainChallenge(row), nil
}

func (s *Store) ConsumeChallenge(ctx context.Context, challengeID uuid.UUID, now time.Time) error {
	return s.db.WithContext(ctx).
		Model(&model.Challenge{}).
		Where("id = ? AND consumed_at IS NULL", challengeID).
		Update("consumed_at", now).Error
}

func (s *Store) IncrementChallengeAttemptCount(ctx context.Context, challengeID uuid.UUID) error {
	return s.db.WithContext(ctx).
		Model(&model.Challenge{}).
		Where("id = ?", challengeID).
		UpdateColumn("attempt_count", gorm.Expr("attempt_count + 1")).Error
}

func (s *Store) IncrementChallengeResendCount(ctx context.Context, challengeID uuid.UUID, now time.Time) (bool, error) {
	result := s.db.WithContext(ctx).
		Model(&model.Challenge{}).
		Where("id = ? AND resend_count < max_resends", challengeID).
		Updates(map[string]any{
			"resend_count": gorm.Expr("resend_count + 1"),
			"last_sent_at": now,
		})

	if result.Error != nil {
		return false, result.Error
	}

	// RowsAffected == 0 means limit reached (or not found)
	return result.RowsAffected == 1, nil
}

func (s *Store) SetChallengeLastSentAt(ctx context.Context, challengeID uuid.UUID, now time.Time) error {
	return s.db.WithContext(ctx).
		Model(&model.Challenge{}).
		Where("id = ?", challengeID).
		Update("last_sent_at", now).
		Error
}

func (s *Store) DeleteSentEmailJobsBefore(ctx context.Context, t time.Time) error {
	return s.db.WithContext(ctx).
		Where("status = ? AND sent_at < ?", string(domain.EmailJobStatusSent), t).
		Delete(&model.EmailJob{}).
		Error
}

func (s *Store) DeleteFailedEmailJobsBefore(ctx context.Context, t time.Time) error {
	return s.db.WithContext(ctx).
		Where("status = ? AND created_at < ?", string(domain.EmailJobStatusFailed), t).
		Delete(&model.EmailJob{}).
		Error
}

func (s *Store) UpsertVerificationCode(ctx context.Context, in domain.VerificationCode) error {
	row := toModelVerificationCode(in)

	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "challenge_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"code_hash":  row.CodeHash,
				"expires_at": row.ExpiresAt,
				"created_at": gorm.Expr("now()"),
			}),
		}).
		Create(&row).Error
}

func (s *Store) GetVerificationCodeByChallengeID(ctx context.Context, challengeID uuid.UUID) (domain.VerificationCode, error) {
	var row model.VerificationCode

	err := s.db.WithContext(ctx).
		Where("challenge_id = ?", challengeID).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.VerificationCode{}, ErrorVerificationCodeNotFound
		}
		return domain.VerificationCode{}, err
	}

	return toDomainVerificationCode(row), nil
}
