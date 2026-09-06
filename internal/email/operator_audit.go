package email

import (
	"context"
	"fmt"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store"
)

type OperatorAuditQuery struct {
	Page     int
	Size     int
	Action   string
	Template domain.EmailTemplate
}

type OperatorAuditPage struct {
	Events   []domain.OperatorAuditEvent
	Page     int
	Size     int
	HasNext  bool
	Action   string
	Template domain.EmailTemplate
}

func (s *TemplateService) ListAuditEvents(ctx context.Context, query OperatorAuditQuery) (OperatorAuditPage, error) {
	page := query.Page
	if page < 1 {
		page = 1
	}
	size := query.Size
	if size <= 0 {
		size = 50
	}
	if size > 100 {
		size = 100
	}
	if query.Template != "" {
		if err := ValidateTemplate(query.Template); err != nil {
			return OperatorAuditPage{}, err
		}
	}
	if query.Action != "" &&
		query.Action != domain.OperatorAuditActionEmailTemplateSaved &&
		query.Action != domain.OperatorAuditActionEmailTemplateRestoredBuiltIn {
		return OperatorAuditPage{}, fmt.Errorf("unknown operator audit action %q", query.Action)
	}

	events, err := s.store.ListOperatorAuditEvents(ctx, store.OperatorAuditEventFilter{
		Action:       query.Action,
		ResourceType: domain.OperatorAuditResourceEmailTemplate,
		ResourceID:   string(query.Template),
		Limit:        size + 1,
		Offset:       (page - 1) * size,
	})
	if err != nil {
		return OperatorAuditPage{}, fmt.Errorf("list operator audit events: %w", err)
	}

	hasNext := len(events) > size
	if hasNext {
		events = events[:size]
	}
	return OperatorAuditPage{
		Events:   events,
		Page:     page,
		Size:     size,
		HasNext:  hasNext,
		Action:   query.Action,
		Template: query.Template,
	}, nil
}
