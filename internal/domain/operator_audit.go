package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	OperatorAuditActionEmailTemplateSaved            = "email_template.saved"
	OperatorAuditActionEmailTemplateRestoredBuiltIn  = "email_template.restored_builtin"
	OperatorAuditActionEmailTemplateDeliveryEnabled  = "email_template.delivery_enabled"
	OperatorAuditActionEmailTemplateDeliveryDisabled = "email_template.delivery_disabled"
	OperatorAuditResourceEmailTemplate               = "email_template"
)

// OperatorAuditEvent records a control-plane mutation without storing the
// edited template source or rendered email contents.
type OperatorAuditEvent struct {
	ID        uuid.UUID
	CreatedAt time.Time

	ActorUserID *uuid.UUID
	ActorEmail  *string

	Action       string
	ResourceType string
	ResourceID   string
	Metadata     json.RawMessage
}
