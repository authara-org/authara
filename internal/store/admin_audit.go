package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/google/uuid"
)

type AdminAuditEventFilter struct {
	ActorUserID  *uuid.UUID
	TargetUserID *uuid.UUID
	Action       string
	TargetEmail  string
	Limit        int
	Offset       int
}

const adminAuditEventColumns = `
	id,
	created_at,
	actor_user_id,
	action,
	target_user_id,
	target_email,
	metadata,
	ip,
	user_agent
`

func scanAdminAuditEvent(row rowScanner, event *domain.AdminAuditEvent) error {
	return row.Scan(
		&event.ID,
		&event.CreatedAt,
		&event.ActorUserID,
		&event.Action,
		&event.TargetUserID,
		&event.TargetEmail,
		&event.Metadata,
		&event.IP,
		&event.UserAgent,
	)
}

func (s *Store) CreateAdminAuditEvent(ctx context.Context, event domain.AdminAuditEvent) (domain.AdminAuditEvent, error) {
	if len(event.Metadata) == 0 {
		event.Metadata = json.RawMessage(`{}`)
	}

	if err := scanAdminAuditEvent(s.queryRow(ctx, `
		INSERT INTO admin_audit_events (
			actor_user_id,
			action,
			target_user_id,
			target_email,
			metadata,
			ip,
			user_agent
		)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7)
		RETURNING `+adminAuditEventColumns,
		event.ActorUserID,
		event.Action,
		event.TargetUserID,
		event.TargetEmail,
		event.Metadata,
		event.IP,
		event.UserAgent,
	), &event); err != nil {
		return domain.AdminAuditEvent{}, err
	}

	return event, nil
}

func (s *Store) ListAdminAuditEvents(ctx context.Context, filter AdminAuditEventFilter) ([]domain.AdminAuditEvent, error) {
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
		SELECT `+adminAuditEventColumns+`
		FROM admin_audit_events
		WHERE ($1::uuid IS NULL OR actor_user_id = $1)
		  AND ($2::uuid IS NULL OR target_user_id = $2)
		  AND ($3 = '' OR action = $3)
		  AND ($4 = '' OR target_email = $4)
		ORDER BY created_at DESC
		LIMIT $5 OFFSET $6
	`, filter.ActorUserID, filter.TargetUserID, filter.Action, filter.TargetEmail, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.AdminAuditEvent, 0)
	for rows.Next() {
		var event domain.AdminAuditEvent
		if err := scanAdminAuditEvent(rows, &event); err != nil {
			return nil, err
		}
		out = append(out, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func (s *Store) DeleteAdminAuditEventsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := s.exec(ctx, `DELETE FROM admin_audit_events WHERE created_at < $1`, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
