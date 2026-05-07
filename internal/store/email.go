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
	"gorm.io/gorm/logger"
)

func toDomainEmailJob(m model.EmailJob) domain.EmailJob {
	return domain.EmailJob{
		ID:                  *m.ID,
		ChallengeID:         m.ChallengeID,
		CreatedAt:           m.CreatedAt,
		UpdatedAt:           m.UpdatedAt,
		ToEmail:             m.ToEmail,
		Template:            domain.EmailTemplate(m.Template),
		TemplateData:        m.TemplateData,
		Status:              domain.EmailJobStatus(m.Status),
		AttemptCount:        m.AttemptCount,
		ProcessingStartedAt: m.ProcessingStartedAt,
		LastError:           m.LastError,
		NextAttemptAt:       m.NextAttemptAt,
		SentAt:              m.SentAt,
	}
}

func toModelEmailJob(d domain.EmailJob) model.EmailJob {
	return model.EmailJob{
		ChallengeID:         d.ChallengeID,
		ToEmail:             d.ToEmail,
		Template:            string(d.Template),
		TemplateData:        d.TemplateData,
		Status:              string(d.Status),
		AttemptCount:        d.AttemptCount,
		ProcessingStartedAt: d.ProcessingStartedAt,
		LastError:           d.LastError,
		NextAttemptAt:       d.NextAttemptAt,
		SentAt:              d.SentAt,
	}
}

func (s *Store) CreateEmailJob(ctx context.Context, in domain.EmailJob) (domain.EmailJob, error) {
	row := toModelEmailJob(in)

	err := s.query(ctx).
		Create(&row).
		Error

	if err != nil {
		return domain.EmailJob{}, err
	}

	return toDomainEmailJob(row), nil
}

func (s *Store) ClaimNextEmailJob(ctx context.Context, now time.Time) (domain.EmailJob, error) {
	var row model.EmailJob

	err := s.db.
		Session(&gorm.Session{
			Logger: logger.Default.LogMode(logger.Silent),
		}).
		WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ? AND next_attempt_at <= ?", string(domain.EmailJobStatusPending), now).
			Order("next_attempt_at ASC, created_at ASC").
			First(&row).Error

		if err != nil {
			return err
		}

		return tx.Model(&model.EmailJob{}).
			Where("id = ?", *row.ID).
			Updates(map[string]any{
				"status":                string(domain.EmailJobStatusProcessing),
				"processing_started_at": now,
			}).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.EmailJob{}, ErrorEmailJobNotFound
		}
		return domain.EmailJob{}, err
	}

	row.Status = string(domain.EmailJobStatusProcessing)
	row.ProcessingStartedAt = &now

	return toDomainEmailJob(row), nil
}

func (s *Store) MarkEmailJobSent(ctx context.Context, jobID uuid.UUID, now time.Time) error {
	return s.query(ctx).
		Model(&model.EmailJob{}).
		Where("id = ?", jobID).
		Updates(map[string]any{
			"status":                string(domain.EmailJobStatusSent),
			"sent_at":               now,
			"processing_started_at": nil,
		}).Error
}

func (s *Store) RequeueEmailJob(ctx context.Context, jobID uuid.UUID, lastError string, nextAttemptAt time.Time) error {
	return s.query(ctx).
		Model(&model.EmailJob{}).
		Where("id = ?", jobID).
		Updates(map[string]any{
			"status":                string(domain.EmailJobStatusPending),
			"attempt_count":         gorm.Expr("attempt_count + 1"),
			"last_error":            lastError,
			"next_attempt_at":       nextAttemptAt,
			"processing_started_at": nil,
		}).Error
}

func (s *Store) MarkEmailJobFailed(ctx context.Context, jobID uuid.UUID, lastError string) error {
	return s.query(ctx).
		Model(&model.EmailJob{}).
		Where("id = ?", jobID).
		Updates(map[string]any{
			"status":                string(domain.EmailJobStatusFailed),
			"attempt_count":         gorm.Expr("attempt_count + 1"),
			"last_error":            lastError,
			"processing_started_at": nil,
		}).Error
}
