package store

import (
	"context"

	"github.com/authara-org/authara/internal/domain"
)

const securityEventColumns = `
	id,
	created_at,
	type,
	user_id,
	passkey_id,
	response
`

func scanSecurityEvent(row rowScanner, event *domain.SecurityEvent) error {
	return row.Scan(
		&event.ID,
		&event.CreatedAt,
		&event.Type,
		&event.UserID,
		&event.PasskeyID,
		&event.Response,
	)
}

func (s *Store) CreateSecurityEvent(ctx context.Context, event domain.SecurityEvent) (domain.SecurityEvent, error) {
	if err := scanSecurityEvent(s.queryRow(ctx, `
		INSERT INTO security_events (type, user_id, passkey_id, response)
		VALUES ($1, $2, $3, $4)
		RETURNING `+securityEventColumns,
		event.Type,
		event.UserID,
		event.PasskeyID,
		event.Response,
	), &event); err != nil {
		return domain.SecurityEvent{}, err
	}
	return event, nil
}

func (s *Store) ListSecurityEvents(ctx context.Context, limit, offset int) ([]domain.SecurityEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := s.queryRows(ctx, `
		SELECT `+securityEventColumns+`
		FROM security_events
		ORDER BY created_at DESC, id DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
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
