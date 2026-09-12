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
	OperatorAuditActionRuntimeSettingSet             = "runtime_setting.set"
	OperatorAuditActionRuntimeSettingCleared         = "runtime_setting.cleared"
	OperatorAuditResourceEmailTemplate               = "email_template"
	OperatorAuditResourceRuntimeSetting              = "runtime_setting"
)

// OperatorAuditEvent records a control-plane mutation. Metadata is limited to
// non-sensitive identifiers and change summaries rather than resource bodies.
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
