package store

import (
	"context"

	"github.com/authara-org/authara/internal/domain"
	"github.com/google/uuid"
)

type OperatorAuditEventFilter struct {
	ActorUserID  *uuid.UUID
	Action       string
	ResourceType string
	ResourceID   string
	Limit        int
	Offset       int
}

const operatorAuditEventColumns = `
	id,
	created_at,
	actor_user_id,
	action,
	resource_type,
	resource_id,
	metadata
`

func scanOperatorAuditEvent(row rowScanner, event *domain.OperatorAuditEvent) error {
	return row.Scan(
		&event.ID,
		&event.CreatedAt,
		&event.ActorUserID,
		&event.Action,
		&event.ResourceType,
		&event.ResourceID,
		&event.Metadata,
	)
}

func scanOperatorAuditEventWithActorEmail(row rowScanner, event *domain.OperatorAuditEvent) error {
	return row.Scan(
		&event.ID,
		&event.CreatedAt,
		&event.ActorUserID,
		&event.Action,
		&event.ResourceType,
		&event.ResourceID,
		&event.Metadata,
		&event.ActorEmail,
	)
}

func (s *Store) ListOperatorAuditEvents(ctx context.Context, filter OperatorAuditEventFilter) ([]domain.OperatorAuditEvent, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	rows, err := s.queryRows(ctx, `
		SELECT
			audit.id,
			audit.created_at,
			audit.actor_user_id,
			audit.action,
			audit.resource_type,
			audit.resource_id,
			audit.metadata,
			actor.email
		FROM operator_audit_events AS audit
		LEFT JOIN users AS actor ON actor.id = audit.actor_user_id
		WHERE ($1::uuid IS NULL OR audit.actor_user_id = $1)
		  AND ($2 = '' OR audit.action = $2)
		  AND ($3 = '' OR audit.resource_type = $3)
		  AND ($4 = '' OR audit.resource_id = $4)
		ORDER BY audit.created_at DESC, audit.id DESC
		LIMIT $5 OFFSET $6
	`, filter.ActorUserID, filter.Action, filter.ResourceType, filter.ResourceID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]domain.OperatorAuditEvent, 0)
	for rows.Next() {
		var event domain.OperatorAuditEvent
		if err := scanOperatorAuditEventWithActorEmail(rows, &event); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}
