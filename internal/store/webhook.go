package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store/model"
)

const webhookEventColumns = `
	id,
	created_at,
	updated_at,
	event_type,
	payload,
	status,
	attempt_count,
	next_attempt_at,
	processing_started_at,
	last_error,
	delivered_at
`

func scanWebhookEvent(row rowScanner, event *model.WebhookEvent) error {
	return row.Scan(
		&event.ID,
		&event.CreatedAt,
		&event.UpdatedAt,
		&event.EventType,
		&event.Payload,
		&event.Status,
		&event.AttemptCount,
		&event.NextAttemptAt,
		&event.ProcessingStartedAt,
		&event.LastError,
		&event.DeliveredAt,
	)
}

func toDomainWebhookEvent(event model.WebhookEvent) domain.WebhookEvent {
	return domain.WebhookEvent{
		ID:                  event.ID,
		CreatedAt:           event.CreatedAt,
		UpdatedAt:           event.UpdatedAt,
		EventType:           event.EventType,
		Payload:             event.Payload,
		Status:              domain.WebhookEventStatus(event.Status),
		AttemptCount:        event.AttemptCount,
		NextAttemptAt:       event.NextAttemptAt,
		ProcessingStartedAt: event.ProcessingStartedAt,
		LastError:           event.LastError,
		DeliveredAt:         event.DeliveredAt,
	}
}

func (s *Store) CreateWebhookEvent(ctx context.Context, event domain.WebhookEvent) (domain.WebhookEvent, error) {
	var row model.WebhookEvent
	err := scanWebhookEvent(s.queryRow(ctx, `
		INSERT INTO webhook_events (id, event_type, payload, status)
		VALUES ($1, $2, $3::jsonb, $4)
		RETURNING `+webhookEventColumns,
		event.ID,
		event.EventType,
		string(event.Payload),
		string(domain.WebhookEventStatusPending),
	), &row)
	if err != nil {
		return domain.WebhookEvent{}, err
	}
	return toDomainWebhookEvent(row), nil
}

func (s *Store) GetWebhookEventByID(ctx context.Context, eventID string) (domain.WebhookEvent, error) {
	var row model.WebhookEvent
	err := scanWebhookEvent(s.queryRow(ctx, `
		SELECT `+webhookEventColumns+`
		FROM webhook_events
		WHERE id = $1
	`, eventID), &row)
	if err != nil {
		return domain.WebhookEvent{}, mapNoRows(err, ErrorWebhookEventNotFound)
	}
	return toDomainWebhookEvent(row), nil
}

func (s *Store) ClaimNextWebhookEvent(ctx context.Context, now time.Time) (domain.WebhookEvent, error) {
	var row model.WebhookEvent

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return domain.WebhookEvent{}, err
	}
	defer func() { _ = tx.Rollback() }()

	err = scanWebhookEvent(tx.QueryRowContext(ctx, `
		SELECT `+webhookEventColumns+`
		FROM webhook_events
		WHERE status = 'pending' AND next_attempt_at <= $1
		ORDER BY next_attempt_at ASC, created_at ASC, id ASC
		FOR UPDATE SKIP LOCKED
		LIMIT 1
	`, now), &row)
	if err != nil {
		return domain.WebhookEvent{}, mapNoRows(err, ErrorWebhookEventNotFound)
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE webhook_events
		SET status = $1,
		    attempt_count = attempt_count + 1,
		    processing_started_at = $2
		WHERE id = $3
	`, string(domain.WebhookEventStatusProcessing), now, row.ID)
	if err != nil {
		return domain.WebhookEvent{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.WebhookEvent{}, err
	}

	row.Status = string(domain.WebhookEventStatusProcessing)
	row.AttemptCount++
	row.ProcessingStartedAt = &now
	return toDomainWebhookEvent(row), nil
}

func (s *Store) MarkWebhookEventDelivered(
	ctx context.Context,
	eventID string,
	processingStartedAt time.Time,
	deliveredAt time.Time,
) error {
	result, err := s.exec(ctx, `
		UPDATE webhook_events
		SET status = $1,
		    delivered_at = $2,
		    processing_started_at = NULL,
		    last_error = NULL
		WHERE id = $3
		  AND status = $4
		  AND processing_started_at = $5
	`,
		string(domain.WebhookEventStatusDelivered),
		deliveredAt,
		eventID,
		string(domain.WebhookEventStatusProcessing),
		processingStartedAt,
	)
	return ensureWebhookEventTransition(result, err)
}

func (s *Store) MarkWebhookEventFailed(
	ctx context.Context,
	eventID string,
	processingStartedAt time.Time,
	lastError string,
) error {
	result, err := s.exec(ctx, `
		UPDATE webhook_events
		SET status = $1,
		    processing_started_at = NULL,
		    last_error = $2
		WHERE id = $3
		  AND status = $4
		  AND processing_started_at = $5
	`,
		string(domain.WebhookEventStatusFailed),
		lastError,
		eventID,
		string(domain.WebhookEventStatusProcessing),
		processingStartedAt,
	)
	return ensureWebhookEventTransition(result, err)
}

func (s *Store) RequeueWebhookEvent(
	ctx context.Context,
	eventID string,
	processingStartedAt time.Time,
	lastError string,
	nextAttemptAt time.Time,
) error {
	result, err := s.exec(ctx, `
		UPDATE webhook_events
		SET status = $1,
		    next_attempt_at = $2,
		    processing_started_at = NULL,
		    last_error = $3
		WHERE id = $4
		  AND status = $5
		  AND processing_started_at = $6
	`,
		string(domain.WebhookEventStatusPending),
		nextAttemptAt,
		lastError,
		eventID,
		string(domain.WebhookEventStatusProcessing),
		processingStartedAt,
	)
	return ensureWebhookEventTransition(result, err)
}

func ensureWebhookEventTransition(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrorWebhookEventLeaseLost
	}
	return nil
}

func (s *Store) ReapStaleWebhookEvents(
	ctx context.Context,
	staleBefore time.Time,
	nextAttemptAt time.Time,
	maxAttempts int,
	batchSize int,
) (int64, error) {
	result, err := s.exec(ctx, `
		WITH stale AS (
			SELECT id
			FROM webhook_events
			WHERE status = 'processing' AND processing_started_at <= $1
			ORDER BY processing_started_at ASC, id ASC
			FOR UPDATE SKIP LOCKED
			LIMIT $2
		)
		UPDATE webhook_events AS event
		SET status = CASE
				WHEN event.attempt_count >= $3 THEN 'failed'
				ELSE 'pending'
			END,
		    next_attempt_at = CASE
				WHEN event.attempt_count >= $3 THEN event.next_attempt_at
				ELSE $4
			END,
		    processing_started_at = NULL,
		    last_error = $5
		FROM stale
		WHERE event.id = stale.id
	`,
		staleBefore,
		batchSize,
		maxAttempts,
		nextAttemptAt,
		"processing lease expired",
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *Store) DeleteExpiredWebhookEvents(
	ctx context.Context, deliveredBefore, failedBefore time.Time, batchSize int,
) (int64, error) {
	result, err := s.exec(ctx, `
		WITH oldest AS (
			SELECT id
			FROM webhook_events
			WHERE (status = 'delivered' AND updated_at < $1)
			   OR (status = 'failed' AND updated_at < $2)
			ORDER BY updated_at ASC, id ASC
			FOR UPDATE SKIP LOCKED
			LIMIT $3
		)
		DELETE FROM webhook_events AS event
		USING oldest
		WHERE event.id = oldest.id
	`,
		deliveredBefore,
		failedBefore,
		batchSize,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
