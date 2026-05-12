package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store/model"
	"github.com/google/uuid"
)

func toDomainEmailJob(m model.EmailJob) domain.EmailJob {
	return domain.EmailJob{
		ID:                  m.ID,
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

const emailJobColumns = `
	id,
	created_at,
	updated_at,
	challenge_id,
	to_email,
	template,
	template_data,
	status,
	attempt_count,
	next_attempt_at,
	processing_started_at,
	last_error,
	sent_at
`

func scanEmailJob(row rowScanner, m *model.EmailJob) error {
	return row.Scan(
		&m.ID,
		&m.CreatedAt,
		&m.UpdatedAt,
		&m.ChallengeID,
		&m.ToEmail,
		&m.Template,
		&m.TemplateData,
		&m.Status,
		&m.AttemptCount,
		&m.NextAttemptAt,
		&m.ProcessingStartedAt,
		&m.LastError,
		&m.SentAt,
	)
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

	if err := scanEmailJob(s.queryRow(ctx, `
		INSERT INTO email_jobs (
			challenge_id,
			to_email,
			template,
			template_data,
			status,
			attempt_count,
			processing_started_at,
			last_error,
			next_attempt_at,
			sent_at
		)
		VALUES ($1, $2, $3, $4::jsonb, $5, $6, $7, $8, $9, $10)
		RETURNING `+emailJobColumns,
		row.ChallengeID,
		row.ToEmail,
		row.Template,
		nullableJSONBytes(row.TemplateData),
		row.Status,
		row.AttemptCount,
		row.ProcessingStartedAt,
		row.LastError,
		row.NextAttemptAt,
		row.SentAt,
	), &row); err != nil {
		return domain.EmailJob{}, err
	}

	return toDomainEmailJob(row), nil
}

func (s *Store) ClaimNextEmailJob(ctx context.Context, now time.Time) (domain.EmailJob, error) {
	var row model.EmailJob

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return domain.EmailJob{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	err = scanEmailJob(tx.QueryRowContext(ctx, `
		SELECT `+emailJobColumns+`
		FROM email_jobs
		WHERE status = $1 AND next_attempt_at <= $2
		ORDER BY next_attempt_at ASC, created_at ASC
		FOR UPDATE SKIP LOCKED
		LIMIT 1
	`, string(domain.EmailJobStatusPending), now), &row)
	if err != nil {
		return domain.EmailJob{}, mapNoRows(err, ErrorEmailJobNotFound)
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE email_jobs
		SET status = $1,
		    processing_started_at = $2
		WHERE id = $3
	`, string(domain.EmailJobStatusProcessing), now, row.ID)
	if err != nil {
		return domain.EmailJob{}, err
	}

	if err := tx.Commit(); err != nil {
		return domain.EmailJob{}, err
	}

	row.Status = string(domain.EmailJobStatusProcessing)
	row.ProcessingStartedAt = &now

	return toDomainEmailJob(row), nil
}

func (s *Store) MarkEmailJobSent(ctx context.Context, jobID uuid.UUID, now time.Time) error {
	_, err := s.exec(ctx, `
		UPDATE email_jobs
		SET status = $1,
		    sent_at = $2,
		    processing_started_at = NULL
		WHERE id = $3
	`, string(domain.EmailJobStatusSent), now, jobID)
	return err
}

func (s *Store) RequeueEmailJob(ctx context.Context, jobID uuid.UUID, lastError string, nextAttemptAt time.Time) error {
	_, err := s.exec(ctx, `
		UPDATE email_jobs
		SET status = $1,
		    attempt_count = attempt_count + 1,
		    last_error = $2,
		    next_attempt_at = $3,
		    processing_started_at = NULL
		WHERE id = $4
	`, string(domain.EmailJobStatusPending), lastError, nextAttemptAt, jobID)
	return err
}

func (s *Store) MarkEmailJobFailed(ctx context.Context, jobID uuid.UUID, lastError string) error {
	_, err := s.exec(ctx, `
		UPDATE email_jobs
		SET status = $1,
		    attempt_count = attempt_count + 1,
		    last_error = $2,
		    processing_started_at = NULL
		WHERE id = $3
	`, string(domain.EmailJobStatusFailed), lastError, jobID)
	return err
}
