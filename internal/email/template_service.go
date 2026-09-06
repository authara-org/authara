package email

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	htmltemplate "html/template"
	"regexp"
	"strings"
	texttemplate "text/template"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store"
	"github.com/google/uuid"
)

var (
	ErrEmptyTemplatePart          = errors.New("email template part is empty")
	ErrMalformedTemplate          = errors.New("email template syntax is malformed")
	ErrUnknownTemplateVariable    = errors.New("unknown email template variable")
	ErrMissingTemplatePlaceholder = errors.New("required email template placeholder is missing")
	ErrInvalidTemplateSubject     = errors.New("email template subject contains a line break")
	ErrMissingTemplateUpdater     = errors.New("email template updater is missing")

	templateVariablePattern = regexp.MustCompile(`\{\{\s*([a-z][a-z0-9_]*)\s*\}\}`)
)

type TemplateSource string

const (
	TemplateSourceBuiltIn  TemplateSource = "built_in"
	TemplateSourceOverride TemplateSource = "override"
)

type TemplateOverrideStore interface {
	GetEmailTemplateOverride(context.Context, domain.EmailTemplate) (domain.EmailTemplateOverride, error)
	ListEmailTemplateOverrides(context.Context) ([]domain.EmailTemplateOverride, error)
	GetEmailTemplateVersion(context.Context, domain.EmailTemplate, int64) (domain.EmailTemplateVersion, error)
	ListEmailTemplateVersions(context.Context, domain.EmailTemplate) ([]domain.EmailTemplateVersion, error)
	UpsertEmailTemplateOverride(context.Context, domain.EmailTemplateOverride, int64) (domain.EmailTemplateOverride, error)
	DeleteEmailTemplateOverride(context.Context, domain.EmailTemplate, int64) error
}

// EffectiveTemplate combines immutable catalog metadata with an optional
// persisted customization.
type EffectiveTemplate struct {
	Definition TemplateDefinition
	Source     TemplateSource
	Override   *domain.EmailTemplateOverride
}

type SaveTemplateOverrideInput struct {
	Template         domain.EmailTemplate
	SubjectTemplate  string
	TextTemplate     string
	HTMLTemplate     string
	ExpectedRevision int64
	UpdatedByUserID  uuid.UUID
}

type PreviewTemplateInput struct {
	Template        domain.EmailTemplate
	SubjectTemplate string
	TextTemplate    string
	HTMLTemplate    string
}

// TemplateService resolves, validates, and renders operator-managed template
// overrides for previews and outgoing email delivery.
type TemplateService struct {
	store TemplateOverrideStore
}

func NewTemplateService(store TemplateOverrideStore) *TemplateService {
	return &TemplateService{store: store}
}

func (s *TemplateService) Get(ctx context.Context, key domain.EmailTemplate) (EffectiveTemplate, error) {
	definition, err := LookupTemplate(key)
	if err != nil {
		return EffectiveTemplate{}, err
	}

	override, err := s.store.GetEmailTemplateOverride(ctx, key)
	if errors.Is(err, store.ErrEmailTemplateOverrideNotFound) {
		return EffectiveTemplate{Definition: definition, Source: TemplateSourceBuiltIn}, nil
	}
	if err != nil {
		return EffectiveTemplate{}, fmt.Errorf("get email template override %q: %w", key, err)
	}
	if err := validateTemplateOverride(definition, override); err != nil {
		return EffectiveTemplate{}, fmt.Errorf("validate stored email template override %q: %w", key, err)
	}

	return EffectiveTemplate{
		Definition: definition,
		Source:     TemplateSourceOverride,
		Override:   &override,
	}, nil
}

func (s *TemplateService) List(ctx context.Context) ([]EffectiveTemplate, error) {
	overrides, err := s.store.ListEmailTemplateOverrides(ctx)
	if err != nil {
		return nil, fmt.Errorf("list email template overrides: %w", err)
	}

	byKey := make(map[domain.EmailTemplate]domain.EmailTemplateOverride, len(overrides))
	for _, override := range overrides {
		definition, err := LookupTemplate(override.Template)
		if err != nil {
			return nil, fmt.Errorf("validate stored email template override: %w", err)
		}
		if err := validateTemplateOverride(definition, override); err != nil {
			return nil, fmt.Errorf("validate stored email template override %q: %w", override.Template, err)
		}
		byKey[override.Template] = override
	}

	catalog := TemplateCatalog()
	out := make([]EffectiveTemplate, 0, len(catalog))
	for _, definition := range catalog {
		effective := EffectiveTemplate{
			Definition: definition,
			Source:     TemplateSourceBuiltIn,
		}
		if override, ok := byKey[definition.Key]; ok {
			effective.Source = TemplateSourceOverride
			effective.Override = &override
		}
		out = append(out, effective)
	}
	return out, nil
}

func (s *TemplateService) SaveOverride(ctx context.Context, in SaveTemplateOverrideInput) (EffectiveTemplate, error) {
	definition, err := LookupTemplate(in.Template)
	if err != nil {
		return EffectiveTemplate{}, err
	}
	if in.UpdatedByUserID == uuid.Nil {
		return EffectiveTemplate{}, ErrMissingTemplateUpdater
	}

	override := domain.EmailTemplateOverride{
		Template:        in.Template,
		SubjectTemplate: in.SubjectTemplate,
		TextTemplate:    in.TextTemplate,
		HTMLTemplate:    in.HTMLTemplate,
		UpdatedByUserID: &in.UpdatedByUserID,
	}
	if err := validateTemplateOverride(definition, override); err != nil {
		return EffectiveTemplate{}, err
	}

	saved, err := s.store.UpsertEmailTemplateOverride(ctx, override, in.ExpectedRevision)
	if err != nil {
		if errors.Is(err, store.ErrEmailTemplateRevisionConflict) {
			return EffectiveTemplate{}, err
		}
		return EffectiveTemplate{}, fmt.Errorf("save email template override %q: %w", in.Template, err)
	}
	return EffectiveTemplate{
		Definition: definition,
		Source:     TemplateSourceOverride,
		Override:   &saved,
	}, nil
}

func (s *TemplateService) DeleteOverride(ctx context.Context, key domain.EmailTemplate, expectedRevision int64) error {
	if err := ValidateTemplate(key); err != nil {
		return err
	}
	if err := s.store.DeleteEmailTemplateOverride(ctx, key, expectedRevision); err != nil {
		if errors.Is(err, store.ErrEmailTemplateRevisionConflict) {
			return err
		}
		return fmt.Errorf("delete email template override %q: %w", key, err)
	}
	return nil
}

func (s *TemplateService) History(ctx context.Context, key domain.EmailTemplate) ([]domain.EmailTemplateVersion, error) {
	if err := ValidateTemplate(key); err != nil {
		return nil, err
	}
	versions, err := s.store.ListEmailTemplateVersions(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("list email template versions %q: %w", key, err)
	}
	for _, version := range versions {
		if version.Template != key {
			return nil, fmt.Errorf("email template history returned version for %q, want %q", version.Template, key)
		}
	}
	return versions, nil
}

func (s *TemplateService) GetVersion(ctx context.Context, key domain.EmailTemplate, version int64) (domain.EmailTemplateVersion, error) {
	if err := ValidateTemplate(key); err != nil {
		return domain.EmailTemplateVersion{}, err
	}
	if version <= 0 {
		return domain.EmailTemplateVersion{}, store.ErrEmailTemplateVersionNotFound
	}

	snapshot, err := s.store.GetEmailTemplateVersion(ctx, key, version)
	if err != nil {
		if errors.Is(err, store.ErrEmailTemplateVersionNotFound) {
			return domain.EmailTemplateVersion{}, err
		}
		return domain.EmailTemplateVersion{}, fmt.Errorf("get email template version %q v%d: %w", key, version, err)
	}
	if snapshot.Template != key {
		return domain.EmailTemplateVersion{}, fmt.Errorf("email template version returned snapshot for %q, want %q", snapshot.Template, key)
	}
	return snapshot, nil
}

func (s *TemplateService) Preview(in PreviewTemplateInput) (Message, error) {
	definition, err := LookupTemplate(in.Template)
	if err != nil {
		return Message{}, err
	}
	// Drafts remain previewable while an operator is moving or rewriting a
	// required placeholder. SaveOverride still validates the full contract.
	definition.RequiredSubjectVariables = nil
	definition.RequiredBodyVariables = nil
	return renderTemplateSources(
		definition,
		in.SubjectTemplate,
		in.TextTemplate,
		in.HTMLTemplate,
		definition.SampleData,
	)
}

func (s *TemplateService) Render(ctx context.Context, key domain.EmailTemplate, data TemplateData) (Message, error) {
	effective, err := s.Get(ctx, key)
	if err != nil {
		return Message{}, err
	}
	if effective.Source == TemplateSourceBuiltIn {
		return RenderBuiltInTemplate(key, data)
	}
	return renderTemplateOverride(effective.Definition, *effective.Override, data)
}

func validateTemplateOverride(definition TemplateDefinition, override domain.EmailTemplateOverride) error {
	_, err := renderTemplateOverride(definition, override, definition.SampleData)
	return err
}

func renderTemplateOverride(definition TemplateDefinition, override domain.EmailTemplateOverride, data TemplateData) (Message, error) {
	return renderTemplateSources(
		definition,
		override.SubjectTemplate,
		override.TextTemplate,
		override.HTMLTemplate,
		data,
	)
}

func renderTemplateSources(
	definition TemplateDefinition,
	subjectSource string,
	textSource string,
	htmlSource string,
	data TemplateData,
) (Message, error) {
	if err := validateTemplateData(definition, data); err != nil {
		return Message{}, err
	}

	allowed := make(map[string]struct{}, len(definition.AvailableVariables))
	for _, variable := range definition.AvailableVariables {
		allowed[variable] = struct{}{}
	}

	subject, err := renderTextTemplatePart(
		TemplatePartSubject,
		subjectSource,
		allowed,
		definition.RequiredSubjectVariables,
		data,
	)
	if err != nil {
		return Message{}, err
	}
	if strings.ContainsAny(subject, "\r\n") {
		return Message{}, annotateTemplateSourceError(
			TemplatePartSubject,
			subjectSource,
			templateErrorAt(strings.IndexAny(subjectSource, "\r\n"), ErrInvalidTemplateSubject),
		)
	}
	textBody, err := renderTextTemplatePart(
		TemplatePartText,
		textSource,
		allowed,
		definition.RequiredBodyVariables,
		data,
	)
	if err != nil {
		return Message{}, err
	}
	htmlBody, err := renderHTMLTemplatePart(
		htmlSource,
		allowed,
		definition.RequiredBodyVariables,
		data,
	)
	if err != nil {
		return Message{}, err
	}

	return Message{Subject: subject, Text: textBody, HTML: htmlBody}, nil
}

func renderTextTemplatePart(
	part TemplatePart,
	source string,
	allowed map[string]struct{},
	required []string,
	data TemplateData,
) (string, error) {
	name := string(part)
	rewritten, err := prepareTemplatePart(name, source, allowed, required)
	if err != nil {
		return "", annotateTemplateSourceError(part, source, err)
	}
	tmpl, err := texttemplate.New(name).Parse(rewritten)
	if err != nil {
		return "", annotateTemplateSourceError(
			part,
			source,
			fmt.Errorf("%w: %s: %v", ErrMalformedTemplate, name, err),
		)
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, map[string]string(data)); err != nil {
		return "", annotateTemplateSourceError(
			part,
			source,
			fmt.Errorf("%w: render email template %s: %w", ErrMalformedTemplate, name, err),
		)
	}
	return out.String(), nil
}

func renderHTMLTemplatePart(
	source string,
	allowed map[string]struct{},
	required []string,
	data TemplateData,
) (string, error) {
	rewritten, err := prepareTemplatePart("HTML", source, allowed, required)
	if err != nil {
		return "", annotateTemplateSourceError(TemplatePartHTML, source, err)
	}
	tmpl, err := htmltemplate.New("HTML").Parse(rewritten)
	if err != nil {
		return "", annotateTemplateSourceError(
			TemplatePartHTML,
			source,
			fmt.Errorf("%w: HTML: %v", ErrMalformedTemplate, err),
		)
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, map[string]string(data)); err != nil {
		return "", annotateTemplateSourceError(
			TemplatePartHTML,
			source,
			fmt.Errorf("%w: render email template HTML: %w", ErrMalformedTemplate, err),
		)
	}
	return out.String(), nil
}

func prepareTemplatePart(name, source string, allowed map[string]struct{}, required []string) (string, error) {
	if strings.TrimSpace(source) == "" {
		return "", templateErrorAt(0, fmt.Errorf("%w: %s", ErrEmptyTemplatePart, name))
	}

	matches := templateVariablePattern.FindAllStringSubmatchIndex(source, -1)
	used := make(map[string]struct{}, len(matches))
	var out strings.Builder
	last := 0
	for _, match := range matches {
		literal := source[last:match[0]]
		if strings.Contains(literal, "{{") || strings.Contains(literal, "}}") {
			return "", templateErrorAt(
				last+malformedDelimiterOffset(literal),
				fmt.Errorf("%w: %s", ErrMalformedTemplate, name),
			)
		}
		variable := source[match[2]:match[3]]
		if _, ok := allowed[variable]; !ok {
			return "", templateErrorAt(
				match[0],
				fmt.Errorf("%w: %s uses %q", ErrUnknownTemplateVariable, name, variable),
			)
		}
		used[variable] = struct{}{}
		out.WriteString(literal)
		out.WriteString(`{{index . "`)
		out.WriteString(variable)
		out.WriteString(`"}}`)
		last = match[1]
	}

	literal := source[last:]
	if strings.Contains(literal, "{{") || strings.Contains(literal, "}}") {
		return "", templateErrorAt(
			last+malformedDelimiterOffset(literal),
			fmt.Errorf("%w: %s", ErrMalformedTemplate, name),
		)
	}
	out.WriteString(literal)

	for _, variable := range required {
		if _, ok := used[variable]; !ok {
			return "", templateErrorAt(
				-1,
				fmt.Errorf(
					"%w: %s requires {{%s}}",
					ErrMissingTemplatePlaceholder,
					name,
					variable,
				),
			)
		}
	}
	return out.String(), nil
}
