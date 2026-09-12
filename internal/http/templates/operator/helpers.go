package operator

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/config"
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

type SettingsPageModel struct {
	Settings   []config.Description
	ErrorKey   config.Key
	Error      string
	DraftValue *string
}

type SettingsGroup struct {
	Name          string
	Settings      []config.Description
	EditableCount int
}

func runtimeSettingGroups(values []config.Description) []SettingsGroup {
	groups := make([]SettingsGroup, 0)
	groupIndexes := make(map[string]int)
	for _, setting := range values {
		index, ok := groupIndexes[setting.Group]
		if !ok {
			index = len(groups)
			groupIndexes[setting.Group] = index
			groups = append(groups, SettingsGroup{Name: setting.Group})
		}
		groups[index].Settings = append(groups[index].Settings, setting)
		if !setting.Locked {
			groups[index].EditableCount++
		}
	}
	prioritized := make([]SettingsGroup, 0, len(groups))
	for _, group := range groups {
		if group.EditableCount > 0 {
			prioritized = append(prioritized, group)
		}
	}
	for _, group := range groups {
		if group.EditableCount == 0 {
			prioritized = append(prioritized, group)
		}
	}
	return prioritized
}

func runtimeSettingGroupID(name string) string {
	return "settings-group-" + strings.ToLower(strings.ReplaceAll(name, " ", "-"))
}

func runtimeSettingGroupContainsKey(group SettingsGroup, key config.Key) bool {
	for _, setting := range group.Settings {
		if setting.Key == key {
			return true
		}
	}
	return false
}

func runtimeSettingCountLabel(count int) string {
	label := " variables"
	if count == 1 {
		label = " variable"
	}
	return strconv.Itoa(count) + label
}

func runtimeSettingLiveLabel(count int) string {
	return strconv.Itoa(count) + " live"
}

func runtimeSettingError(model SettingsPageModel, setting config.Description) string {
	if model.ErrorKey == setting.Key {
		return model.Error
	}
	return ""
}

func runtimeSettingInputValue(model SettingsPageModel, setting config.Description) string {
	if model.ErrorKey == setting.Key && model.DraftValue != nil {
		return *model.DraftValue
	}
	return setting.EffectiveValue
}

func runtimeSettingHref(key config.Key) string {
	return "/auth/operator/settings/" + string(key)
}

func runtimeSettingClearHref(key config.Key) string {
	return runtimeSettingHref(key) + "/clear"
}

func runtimeSettingSourceLabel(source config.Source) string {
	switch source {
	case config.SourceEnvironment:
		return "Set · environment"
	case config.SourceOperator:
		return "Set · operator override"
	case config.SourceUnset:
		return "Not set"
	default:
		return "Not set · default used"
	}
}

func runtimeSettingSourceClass(source config.Source) string {
	base := "inline-flex rounded-full px-2 py-0.5 text-xs font-medium"
	switch source {
	case config.SourceEnvironment:
		return base + " bg-purple-50 text-purple-800 dark:bg-purple-900/30 dark:text-purple-200"
	case config.SourceOperator:
		return base + " bg-green-50 text-green-800 dark:bg-green-900/30 dark:text-green-200"
	case config.SourceUnset:
		return base + " bg-grey-100 text-grey-700 dark:bg-grey-800 dark:text-grey-200"
	default:
		return base + " bg-blue-50 text-blue-800 dark:bg-blue-900/30 dark:text-blue-200"
	}
}

func runtimeSettingMutabilityLabel(setting config.Description) string {
	if setting.Locked {
		return "Deployment only"
	}
	return "Editable here"
}

func runtimeSettingMutabilityClass(setting config.Description) string {
	base := "inline-flex rounded-full px-2 py-0.5 text-xs font-medium"
	if setting.Locked {
		return base + " bg-grey-100 text-grey-700 dark:bg-grey-800 dark:text-grey-200"
	}
	return base + " bg-green-50 text-green-800 dark:bg-green-900/30 dark:text-green-200"
}

func runtimeSettingStateDescription(setting config.Description) string {
	switch setting.EffectiveSource {
	case config.SourceEnvironment:
		if setting.Sensitive {
			return "Provided by the environment. The value is hidden."
		}
		return "Provided by the environment."
	case config.SourceOperator:
		return "A saved operator override is active."
	case config.SourceUnset:
		if setting.Required {
			return "No value is configured, although this variable is required."
		}
		return "No value is configured and there is no built-in default."
	default:
		return "Not provided in the environment; the built-in default is active."
	}
}

func runtimeSettingDisplayValue(setting config.Description) string {
	if setting.Sensitive && setting.EffectiveSource == config.SourceEnvironment {
		return "Configured (value hidden)"
	}
	if setting.EffectiveSource == config.SourceUnset || setting.EffectiveValue == "" {
		return "—"
	}
	return setting.EffectiveValue
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

func emailTemplateDeliveryHref(key domain.EmailTemplate) string {
	return emailTemplateHref(key) + "/delivery"
}

func emailTemplateDeliveryLabel(key domain.EmailTemplate) string {
	definition, err := email.LookupTemplate(key)
	if err != nil {
		return "Email delivery"
	}
	return "Delivery of " + definition.DisplayName
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

func emailTemplateHistoryDropdownValue(model EmailTemplateEditorModel) string {
	if model.ViewingVersion > 0 {
		return emailTemplateHistoryTargetHref(model, model.ViewingVersion)
	}
	if model.Source == email.TemplateSourceOverride && model.ActiveVersion > 0 {
		return emailTemplateHref(model.Definition.Key)
	}
	return ""
}

func emailTemplateHistoryDropdownOptions(model EmailTemplateEditorModel) []dropdown.Option {
	if len(model.Versions) == 0 {
		return []dropdown.Option{{Value: "", Label: "No saved versions"}}
	}

	options := make([]dropdown.Option, 0, len(model.Versions))
	for _, version := range model.Versions {
		label := "Version " + strconv.FormatInt(version.Version, 10) + " - " + emailTemplateVersionTime(version.CreatedAt)
		switch {
		case version.Version == model.ViewingVersion:
			label += " (viewing)"
		case model.Source == email.TemplateSourceOverride && version.Version == model.ActiveVersion:
			label += " (current)"
		}
		options = append(options, dropdown.Option{
			Value: emailTemplateHistoryTargetHref(model, version.Version),
			Label: label,
		})
	}
	return options
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
		{Value: domain.OperatorAuditActionEmailTemplateDeliveryEnabled, Label: "Delivery enabled"},
		{Value: domain.OperatorAuditActionEmailTemplateDeliveryDisabled, Label: "Delivery disabled"},
		{Value: domain.OperatorAuditActionRuntimeSettingSet, Label: "Runtime setting updated"},
		{Value: domain.OperatorAuditActionRuntimeSettingCleared, Label: "Runtime override cleared"},
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
	case domain.OperatorAuditActionEmailTemplateDeliveryEnabled:
		return "Delivery enabled"
	case domain.OperatorAuditActionEmailTemplateDeliveryDisabled:
		return "Delivery disabled"
	case domain.OperatorAuditActionRuntimeSettingSet:
		return "Setting updated"
	case domain.OperatorAuditActionRuntimeSettingCleared:
		return "Override cleared"
	default:
		return action
	}
}

func operatorAuditResourceLabel(event domain.OperatorAuditEvent) string {
	if event.ResourceType == domain.OperatorAuditResourceRuntimeSetting {
		if definition, ok := config.LookupDefinition(config.Key(event.ResourceID)); ok {
			return definition.Name
		}
		return event.ResourceID
	}
	return operatorAuditTemplateLabel(event.ResourceID)
}

func operatorAuditResourceHref(event domain.OperatorAuditEvent) string {
	if event.ResourceType == domain.OperatorAuditResourceRuntimeSetting {
		return "/auth/operator/settings#setting-" + event.ResourceID
	}
	return emailTemplateHref(domain.EmailTemplate(event.ResourceID))
}

func operatorAuditTemplateLabel(resourceID string) string {
	definition, err := email.LookupTemplate(domain.EmailTemplate(resourceID))
	if err != nil {
		return resourceID
	}
	return definition.DisplayName
}

func operatorAuditActorLabel(email *string, actor *uuid.UUID) string {
	if email != nil && *email != "" {
		return *email
	}
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
