package operator

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/email"
	"github.com/authara-org/authara/internal/http/templates/components/dropdown"
	"github.com/google/uuid"
)

type EmailTemplateEditorModel struct {
	Definition     email.TemplateDefinition
	Source         email.TemplateSource
	Revision       int64
	ActiveVersion  int64
	ViewingVersion int64
	Versions       []domain.EmailTemplateVersion

	SubjectTemplate string
	TextTemplate    string
	HTMLTemplate    string
	Preview         *email.Message
	Diagnostic      *EmailTemplateDiagnostic

	Error    string
	Conflict bool
}

type EmailTemplateDiagnostic struct {
	Part    email.TemplatePart
	Line    int
	Message string
}

func emailTemplateHref(key domain.EmailTemplate) string {
	return "/auth/operator/emails/" + string(key)
}

func previewEmailTemplateHref(key domain.EmailTemplate) string {
	return emailTemplateHref(key) + "/preview"
}

func resetEmailTemplateHref(key domain.EmailTemplate) string {
	return emailTemplateHref(key) + "/reset"
}

func emailTemplateVersionHref(key domain.EmailTemplate, version int64) string {
	return emailTemplateHref(key) + "/versions/" + strconv.FormatInt(version, 10)
}

func emailTemplateHistoryTargetHref(model EmailTemplateEditorModel, version int64) string {
	if model.Source == email.TemplateSourceOverride && version == model.ActiveVersion {
		return emailTemplateHref(model.Definition.Key)
	}
	return emailTemplateVersionHref(model.Definition.Key, version)
}

func saveEmailTemplateLabel(model EmailTemplateEditorModel) string {
	if model.ViewingVersion > 0 {
		return "Save as new version"
	}
	return "Save"
}

func emailTemplateVersionTime(value time.Time) string {
	if value.IsZero() {
		return "Saved version"
	}
	return value.UTC().Format("2006-01-02 15:04 UTC")
}

func emailTemplateDropdownOptions() []dropdown.Option {
	definitions := email.TemplateCatalog()
	options := make([]dropdown.Option, 0, len(definitions))
	for _, definition := range definitions {
		options = append(options, dropdown.Option{
			Value: emailTemplateHref(definition.Key),
			Label: definition.DisplayName,
		})
	}
	return options
}

func operatorAuditActionDropdownOptions() []dropdown.Option {
	return []dropdown.Option{
		{Value: "", Label: "All actions"},
		{Value: domain.OperatorAuditActionEmailTemplateSaved, Label: "Template saved"},
		{Value: domain.OperatorAuditActionEmailTemplateRestoredBuiltIn, Label: "Built-in restored"},
	}
}

func operatorAuditTemplateDropdownOptions() []dropdown.Option {
	definitions := email.TemplateCatalog()
	options := make([]dropdown.Option, 0, len(definitions)+1)
	options = append(options, dropdown.Option{Value: "", Label: "All templates"})
	for _, definition := range definitions {
		options = append(options, dropdown.Option{
			Value: string(definition.Key),
			Label: definition.DisplayName,
		})
	}
	return options
}

type operatorAuditMetadata struct {
	Revision int64 `json:"revision"`
	Version  int64 `json:"version"`
}

func operatorAuditActionLabel(action string) string {
	switch action {
	case domain.OperatorAuditActionEmailTemplateSaved:
		return "Template saved"
	case domain.OperatorAuditActionEmailTemplateRestoredBuiltIn:
		return "Built-in restored"
	default:
		return action
	}
}

func operatorAuditTemplateLabel(resourceID string) string {
	definition, err := email.LookupTemplate(domain.EmailTemplate(resourceID))
	if err != nil {
		return resourceID
	}
	return definition.DisplayName
}

func operatorAuditActorLabel(actor *uuid.UUID) string {
	if actor == nil {
		return "Deleted user"
	}
	value := actor.String()
	if len(value) <= 8 {
		return value
	}
	return value[:8]
}

func operatorAuditActorTitle(actor *uuid.UUID) string {
	if actor == nil {
		return "The operator account has been deleted"
	}
	return actor.String()
}

func operatorAuditTime(value time.Time) string {
	return value.UTC().Format("2006-01-02 15:04:05 UTC")
}

func operatorAuditTimeValue(value time.Time) string {
	return value.UTC().Format(time.RFC3339)
}

func operatorAuditMetadataValue(raw json.RawMessage, field string) string {
	var metadata operatorAuditMetadata
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return "—"
	}
	var value int64
	switch field {
	case "revision":
		value = metadata.Revision
	case "version":
		value = metadata.Version
	}
	if value <= 0 {
		return "—"
	}
	return strconv.FormatInt(value, 10)
}

func operatorAuditPageHref(page email.OperatorAuditPage, targetPage int) string {
	values := url.Values{}
	if targetPage > 1 {
		values.Set("page", strconv.Itoa(targetPage))
	}
	if page.Size != 50 {
		values.Set("size", strconv.Itoa(page.Size))
	}
	if page.Action != "" {
		values.Set("action", page.Action)
	}
	if page.Template != "" {
		values.Set("template", string(page.Template))
	}
	if len(values) == 0 {
		return "/auth/operator/audit"
	}
	return "/auth/operator/audit?" + values.Encode()
}

func placeholderList(variables []string) string {
	values := make([]string, len(variables))
	for i, variable := range variables {
		values[i] = "{{" + variable + "}}"
	}
	return strings.Join(values, ", ")
}

func navAriaCurrent(active bool) string {
	if active {
		return "page"
	}
	return "false"
}

func navClass(active bool) string {
	base := "rounded-lg border px-3 py-2 font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 dark:focus:ring-offset-grey-950"
	if active {
		return base + " border-blue-600 bg-blue-600 text-white"
	}
	return base + " border-grey-200 bg-white text-grey-700 hover:border-grey-300 hover:bg-grey-50 dark:border-grey-800 dark:bg-grey-900 dark:text-grey-200 dark:hover:border-grey-700 dark:hover:bg-grey-800"
}
