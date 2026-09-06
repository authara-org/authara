package operator

import (
	"strconv"
	"strings"
	"time"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/email"
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
