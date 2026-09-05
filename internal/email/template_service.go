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
	ErrEmptyTemplatePart       = errors.New("email template part is empty")
	ErrMalformedTemplate       = errors.New("email template syntax is malformed")
	ErrUnknownTemplateVariable = errors.New("unknown email template variable")
	ErrMissingTemplateUpdater  = errors.New("email template updater is missing")

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
	UpsertEmailTemplateOverride(context.Context, domain.EmailTemplateOverride) (domain.EmailTemplateOverride, error)
	DeleteEmailTemplateOverride(context.Context, domain.EmailTemplate) error
}

// EffectiveTemplate combines immutable catalog metadata with an optional
// persisted customization.
type EffectiveTemplate struct {
	Definition TemplateDefinition
	Source     TemplateSource
	Override   *domain.EmailTemplateOverride
}

type SaveTemplateOverrideInput struct {
	Template        domain.EmailTemplate
	SubjectTemplate string
	TextTemplate    string
	HTMLTemplate    string
	UpdatedByUserID uuid.UUID
}

// TemplateService resolves and validates operator-managed template overrides.
// Email delivery is wired to this service in Stage 6.
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

	saved, err := s.store.UpsertEmailTemplateOverride(ctx, override)
	if err != nil {
		return EffectiveTemplate{}, fmt.Errorf("save email template override %q: %w", in.Template, err)
	}
	return EffectiveTemplate{
		Definition: definition,
		Source:     TemplateSourceOverride,
		Override:   &saved,
	}, nil
}

func (s *TemplateService) DeleteOverride(ctx context.Context, key domain.EmailTemplate) error {
	if err := ValidateTemplate(key); err != nil {
		return err
	}
	if err := s.store.DeleteEmailTemplateOverride(ctx, key); err != nil {
		return fmt.Errorf("delete email template override %q: %w", key, err)
	}
	return nil
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
	if err := validateTemplateData(definition, data); err != nil {
		return Message{}, err
	}

	allowed := make(map[string]struct{}, len(definition.RequiredVariables)+len(definition.OptionalVariables))
	for _, variable := range definition.RequiredVariables {
		allowed[variable] = struct{}{}
	}
	for _, variable := range definition.OptionalVariables {
		allowed[variable] = struct{}{}
	}

	subject, err := renderTextTemplatePart("subject", override.SubjectTemplate, allowed, data)
	if err != nil {
		return Message{}, err
	}
	textBody, err := renderTextTemplatePart("text", override.TextTemplate, allowed, data)
	if err != nil {
		return Message{}, err
	}
	htmlBody, err := renderHTMLTemplatePart(override.HTMLTemplate, allowed, data)
	if err != nil {
		return Message{}, err
	}

	return Message{Subject: subject, Text: textBody, HTML: htmlBody}, nil
}

func renderTextTemplatePart(name, source string, allowed map[string]struct{}, data TemplateData) (string, error) {
	rewritten, err := prepareTemplatePart(name, source, allowed)
	if err != nil {
		return "", err
	}
	tmpl, err := texttemplate.New(name).Parse(rewritten)
	if err != nil {
		return "", fmt.Errorf("%w: %s: %v", ErrMalformedTemplate, name, err)
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, map[string]string(data)); err != nil {
		return "", fmt.Errorf("render email template %s: %w", name, err)
	}
	return out.String(), nil
}

func renderHTMLTemplatePart(source string, allowed map[string]struct{}, data TemplateData) (string, error) {
	rewritten, err := prepareTemplatePart("HTML", source, allowed)
	if err != nil {
		return "", err
	}
	tmpl, err := htmltemplate.New("HTML").Parse(rewritten)
	if err != nil {
		return "", fmt.Errorf("%w: HTML: %v", ErrMalformedTemplate, err)
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, map[string]string(data)); err != nil {
		return "", fmt.Errorf("render email template HTML: %w", err)
	}
	return out.String(), nil
}

func prepareTemplatePart(name, source string, allowed map[string]struct{}) (string, error) {
	if strings.TrimSpace(source) == "" {
		return "", fmt.Errorf("%w: %s", ErrEmptyTemplatePart, name)
	}

	matches := templateVariablePattern.FindAllStringSubmatchIndex(source, -1)
	var out strings.Builder
	last := 0
	for _, match := range matches {
		literal := source[last:match[0]]
		if strings.Contains(literal, "{{") || strings.Contains(literal, "}}") {
			return "", fmt.Errorf("%w: %s", ErrMalformedTemplate, name)
		}
		variable := source[match[2]:match[3]]
		if _, ok := allowed[variable]; !ok {
			return "", fmt.Errorf("%w: %s uses %q", ErrUnknownTemplateVariable, name, variable)
		}
		out.WriteString(literal)
		out.WriteString(`{{index . "`)
		out.WriteString(variable)
		out.WriteString(`"}}`)
		last = match[1]
	}

	literal := source[last:]
	if strings.Contains(literal, "{{") || strings.Contains(literal, "}}") {
		return "", fmt.Errorf("%w: %s", ErrMalformedTemplate, name)
	}
	out.WriteString(literal)
	return out.String(), nil
}
