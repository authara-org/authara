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
		WHERE status = $1
		ORDER BY created_at ASC, id ASC
		FOR UPDATE SKIP LOCKED
		LIMIT 1
	`, string(domain.WebhookEventStatusPending)), &row)
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

func (s *Store) MarkWebhookEventDelivered(ctx context.Context, eventID string, now time.Time) error {
	_, err := s.exec(ctx, `
		UPDATE webhook_events
		SET status = $1,
		    delivered_at = $2,
		    processing_started_at = NULL,
		    last_error = NULL
		WHERE id = $3
	`, string(domain.WebhookEventStatusDelivered), now, eventID)
	return err
}

func (s *Store) MarkWebhookEventFailed(ctx context.Context, eventID string, lastError string) error {
	_, err := s.exec(ctx, `
		UPDATE webhook_events
		SET status = $1,
		    processing_started_at = NULL,
		    last_error = $2
		WHERE id = $3
	`, string(domain.WebhookEventStatusFailed), lastError, eventID)
	return err
}
