package store

import (
	"context"
	"database/sql"
	"errors"
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
		DeliveryDeadlineAt:  m.DeliveryDeadlineAt,
		TerminalReason:      m.TerminalReason,
		FailedAt:            m.FailedAt,
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
	delivery_deadline_at,
	processing_started_at,
	last_error,
	terminal_reason,
	failed_at,
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
		&m.DeliveryDeadlineAt,
		&m.ProcessingStartedAt,
		&m.LastError,
		&m.TerminalReason,
		&m.FailedAt,
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
		DeliveryDeadlineAt:  d.DeliveryDeadlineAt,
		TerminalReason:      d.TerminalReason,
		FailedAt:            d.FailedAt,
		SentAt:              d.SentAt,
	}
}

// CreateEmailJob inserts a job when delivery for its template is enabled.
// Missing delivery settings default to enabled; a disabled template returns a
// zero job without an error so the surrounding business transaction can commit.
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
			delivery_deadline_at,
			terminal_reason,
			failed_at,
			sent_at
		)
		SELECT $1, $2, $3::varchar, $4::jsonb, $5, $6, $7, $8, $9,
		       COALESCE(NULLIF($10, '0001-01-01T00:00:00Z'::timestamptz), now() + interval '72 hours'),
		       $11, $12, $13
		WHERE COALESCE((
			SELECT enabled
			FROM email_template_delivery_settings
			WHERE template_key = $3::varchar
		), true)
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
		row.DeliveryDeadlineAt,
		row.TerminalReason,
		row.FailedAt,
		row.SentAt,
	), &row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.EmailJob{}, nil
		}
		return domain.EmailJob{}, err
	}

	return toDomainEmailJob(row), nil
}

func (s *Store) GetEmailJobByID(ctx context.Context, jobID uuid.UUID) (domain.EmailJob, error) {
	var row model.EmailJob
	if err := scanEmailJob(s.queryRow(ctx, `
		SELECT `+emailJobColumns+`
		FROM email_jobs
		WHERE id = $1
	`, jobID), &row); err != nil {
		return domain.EmailJob{}, mapNoRows(err, ErrorEmailJobNotFound)
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
		    attempt_count = attempt_count + 1,
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
	row.AttemptCount++
	row.ProcessingStartedAt = &now

	return toDomainEmailJob(row), nil
}

func (s *Store) MarkEmailJobSent(ctx context.Context, jobID uuid.UUID, processingStartedAt, now time.Time) error {
	result, err := s.exec(ctx, `
		UPDATE email_jobs
		SET status = $1,
		    sent_at = $2,
		    processing_started_at = NULL,
		    last_error = NULL,
		    terminal_reason = NULL,
		    failed_at = NULL
		WHERE id = $3
		  AND status = $4
		  AND processing_started_at = $5
	`, string(domain.EmailJobStatusSent), now, jobID, string(domain.EmailJobStatusProcessing), processingStartedAt)
	return ensureEmailJobTransition(result, err)
}

func (s *Store) RequeueEmailJob(ctx context.Context, jobID uuid.UUID, processingStartedAt time.Time, lastError string, nextAttemptAt time.Time) error {
	result, err := s.exec(ctx, `
		UPDATE email_jobs
		SET status = $1,
		    last_error = $2,
		    next_attempt_at = $3,
		    processing_started_at = NULL,
		    terminal_reason = NULL,
		    failed_at = NULL
		WHERE id = $4
		  AND status = $5
		  AND processing_started_at = $6
	`, string(domain.EmailJobStatusPending), lastError, nextAttemptAt, jobID, string(domain.EmailJobStatusProcessing), processingStartedAt)
	return ensureEmailJobTransition(result, err)
}

func (s *Store) MarkEmailJobFailed(
	ctx context.Context,
	jobID uuid.UUID,
	processingStartedAt time.Time,
	lastError string,
	terminalReason string,
	failedAt time.Time,
) error {
	result, err := s.exec(ctx, `
		UPDATE email_jobs
		SET status = $1,
		    last_error = $2,
		    processing_started_at = NULL,
		    terminal_reason = $3,
		    failed_at = $4
		WHERE id = $5
		  AND status = $6
		  AND processing_started_at = $7
	`, string(domain.EmailJobStatusFailed), lastError, terminalReason, failedAt, jobID, string(domain.EmailJobStatusProcessing), processingStartedAt)
	return ensureEmailJobTransition(result, err)
}

func ensureEmailJobTransition(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrorEmailJobLeaseLost
	}
	return nil
}

func (s *Store) ReapStaleEmailJobs(
	ctx context.Context,
	staleBefore time.Time,
	now time.Time,
	maxAttempts int,
	batchSize int,
	retryBaseDelay time.Duration,
	retryMaxDelay time.Duration,
) (ReapResult, error) {
	var result ReapResult
	err := s.queryRow(ctx, `
		WITH stale AS (
			SELECT id
			FROM email_jobs
			WHERE status = 'processing'
			  AND processing_started_at <= $1
			ORDER BY processing_started_at ASC, id ASC
			FOR UPDATE SKIP LOCKED
			LIMIT $2
		), updated AS (
		UPDATE email_jobs AS job
		SET status = CASE
				WHEN job.attempt_count >= $3 OR job.delivery_deadline_at <= $4 THEN 'failed'
				ELSE 'pending'
			END,
		    next_attempt_at = CASE
				WHEN job.attempt_count >= $3 OR job.delivery_deadline_at <= $4 THEN job.next_attempt_at
				ELSE LEAST(
					job.delivery_deadline_at,
					$4 + make_interval(secs => LEAST(
						$6,
						$5 * power(2, LEAST(GREATEST(job.attempt_count - 1, 0), 30))
					) * (0.5 + random() * 0.5))
				)
			END,
		    processing_started_at = NULL,
		    last_error = 'processing lease expired',
		    terminal_reason = CASE
				WHEN job.delivery_deadline_at <= $4 THEN 'delivery_deadline_exceeded'
				WHEN job.attempt_count >= $3 THEN 'attempts_exhausted'
				ELSE NULL
			END,
		    failed_at = CASE
				WHEN job.attempt_count >= $3 OR job.delivery_deadline_at <= $4 THEN $4
				ELSE NULL
			END
		FROM stale
		WHERE job.id = stale.id
		RETURNING job.status
		)
		SELECT
			count(*) FILTER (WHERE status = 'pending'),
			count(*) FILTER (WHERE status = 'failed')
		FROM updated
	`,
		staleBefore,
		batchSize,
		maxAttempts,
		now,
		retryBaseDelay.Seconds(),
		retryMaxDelay.Seconds(),
	).Scan(&result.Retried, &result.Failed)
	if err != nil {
		return ReapResult{}, err
	}
	return result, nil
}

func (s *Store) ListActiveOrFailedEmailJobs(ctx context.Context, limit, offset int) ([]domain.EmailJob, error) {
	rows, err := s.queryRows(ctx, `
		SELECT `+emailJobColumns+`
		FROM email_jobs
		WHERE status <> $1
		ORDER BY updated_at DESC, created_at DESC
		LIMIT $2 OFFSET $3
	`, string(domain.EmailJobStatusSent), limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.EmailJob, 0)
	for rows.Next() {
		var row model.EmailJob
		if err := scanEmailJob(rows, &row); err != nil {
			return nil, err
		}
		out = append(out, toDomainEmailJob(row))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func (s *Store) CountActiveOrFailedEmailJobs(ctx context.Context) (int, error) {
	var count int
	err := s.queryRow(ctx, `
		SELECT count(*)
		FROM email_jobs
		WHERE status <> $1
	`, string(domain.EmailJobStatusSent)).Scan(&count)
	return count, err
}
