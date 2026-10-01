package store

import (
	"context"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/google/uuid"
)

type SecurityEventFilter struct {
	Type            domain.SecurityEventType
	Outcome         domain.SecurityEventOutcome
	UserID          *uuid.UUID
	SessionID       *uuid.UUID
	From            *time.Time
	Before          *time.Time
	CursorCreatedAt *time.Time
	CursorID        *uuid.UUID
	Limit           int
	Offset          int
}

const securityEventColumns = `
	id,
	created_at,
	type,
	outcome,
	COALESCE(reason_code, ''),
	actor_type,
	actor_user_id,
	user_id,
	session_id,
	organization_id,
	passkey_id,
	COALESCE(authentication_method, ''),
	COALESCE(response, '')
`

func scanSecurityEvent(row rowScanner, event *domain.SecurityEvent) error {
	return row.Scan(
		&event.ID,
		&event.CreatedAt,
		&event.Type,
		&event.Outcome,
		&event.ReasonCode,
		&event.ActorType,
		&event.ActorUserID,
		&event.UserID,
		&event.SessionID,
		&event.OrganizationID,
		&event.PasskeyID,
		&event.AuthenticationMethod,
		&event.Response,
	)
}

func (s *Store) CreateSecurityEvent(ctx context.Context, event domain.SecurityEvent) (domain.SecurityEvent, error) {
	if err := scanSecurityEvent(s.queryRow(ctx, `
		INSERT INTO security_events (
			type, outcome, reason_code, actor_type, actor_user_id, user_id,
			session_id, organization_id, passkey_id, authentication_method, response
		)
		VALUES ($1, $2, NULLIF($3, ''), $4, $5, $6, $7, $8, $9, NULLIF($10, ''), NULLIF($11, ''))
		RETURNING `+securityEventColumns,
		event.Type,
		event.Outcome,
		event.ReasonCode,
		event.ActorType,
		event.ActorUserID,
		event.UserID,
		event.SessionID,
		event.OrganizationID,
		event.PasskeyID,
		event.AuthenticationMethod,
		event.Response,
	), &event); err != nil {
		return domain.SecurityEvent{}, err
	}
	return event, nil
}

func (s *Store) QuerySecurityEvents(ctx context.Context, filter SecurityEventFilter) ([]domain.SecurityEvent, error) {
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
	cursorCreatedAt, cursorID := filter.CursorCreatedAt, filter.CursorID
	if cursorCreatedAt == nil || cursorID == nil {
		cursorCreatedAt, cursorID = nil, nil
	}

	rows, err := s.queryRows(ctx, `
		SELECT `+securityEventColumns+`
		FROM security_events
		WHERE ($1 = '' OR type = $1)
		  AND ($2 = '' OR outcome = $2)
		  AND ($3::uuid IS NULL OR user_id = $3)
		  AND ($4::uuid IS NULL OR session_id = $4)
		  AND ($5::timestamptz IS NULL OR created_at >= $5)
		  AND ($6::timestamptz IS NULL OR created_at < $6)
		  AND ($7::timestamptz IS NULL OR (created_at, id) < ($7, $8::uuid))
		ORDER BY created_at DESC, id DESC
		LIMIT $9 OFFSET $10
	`, filter.Type, filter.Outcome, filter.UserID, filter.SessionID, filter.From, filter.Before,
		cursorCreatedAt, cursorID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]domain.SecurityEvent, 0)
	for rows.Next() {
		var event domain.SecurityEvent
		if err := scanSecurityEvent(rows, &event); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func (s *Store) ListSecurityEvents(ctx context.Context, limit, offset int) ([]domain.SecurityEvent, error) {
	return s.QuerySecurityEvents(ctx, SecurityEventFilter{Limit: limit, Offset: offset})
}

func (s *Store) DeleteSecurityEventsBefore(ctx context.Context, cutoff time.Time, batchSize int) (int64, error) {
	result, err := s.exec(ctx, `
		WITH oldest AS (
			SELECT id
			FROM security_events
			WHERE created_at < $1
			ORDER BY created_at, id
			FOR UPDATE SKIP LOCKED
			LIMIT $2
		)
		DELETE FROM security_events AS event
		USING oldest
		WHERE event.id = oldest.id
	`, cutoff, batchSize)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
