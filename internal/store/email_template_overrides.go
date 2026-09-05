package store

import (
	"context"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store/model"
)

const emailTemplateOverrideColumns = `
	template_key,
	created_at,
	updated_at,
	subject_template,
	text_template,
	html_template,
	revision,
	updated_by_user_id
`

func scanEmailTemplateOverride(row rowScanner, m *model.EmailTemplateOverride) error {
	return row.Scan(
		&m.TemplateKey,
		&m.CreatedAt,
		&m.UpdatedAt,
		&m.SubjectTemplate,
		&m.TextTemplate,
		&m.HTMLTemplate,
		&m.Revision,
		&m.UpdatedByUserID,
	)
}

func toDomainEmailTemplateOverride(m model.EmailTemplateOverride) domain.EmailTemplateOverride {
	return domain.EmailTemplateOverride{
		Template:        domain.EmailTemplate(m.TemplateKey),
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
		SubjectTemplate: m.SubjectTemplate,
		TextTemplate:    m.TextTemplate,
		HTMLTemplate:    m.HTMLTemplate,
		Revision:        m.Revision,
		UpdatedByUserID: m.UpdatedByUserID,
	}
}

func (s *Store) GetEmailTemplateOverride(ctx context.Context, key domain.EmailTemplate) (domain.EmailTemplateOverride, error) {
	var row model.EmailTemplateOverride
	if err := scanEmailTemplateOverride(s.queryRow(ctx, `
		SELECT `+emailTemplateOverrideColumns+`
		FROM email_template_overrides
		WHERE template_key = $1
	`, string(key)), &row); err != nil {
		return domain.EmailTemplateOverride{}, mapNoRows(err, ErrEmailTemplateOverrideNotFound)
	}
	return toDomainEmailTemplateOverride(row), nil
}

func (s *Store) ListEmailTemplateOverrides(ctx context.Context) ([]domain.EmailTemplateOverride, error) {
	rows, err := s.queryRows(ctx, `
		SELECT `+emailTemplateOverrideColumns+`
		FROM email_template_overrides
		ORDER BY template_key ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.EmailTemplateOverride, 0)
	for rows.Next() {
		var row model.EmailTemplateOverride
		if err := scanEmailTemplateOverride(rows, &row); err != nil {
			return nil, err
		}
		out = append(out, toDomainEmailTemplateOverride(row))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) UpsertEmailTemplateOverride(ctx context.Context, in domain.EmailTemplateOverride) (domain.EmailTemplateOverride, error) {
	var row model.EmailTemplateOverride
	if err := scanEmailTemplateOverride(s.queryRow(ctx, `
		INSERT INTO email_template_overrides (
			template_key,
			subject_template,
			text_template,
			html_template,
			updated_by_user_id
		)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (template_key) DO UPDATE
		SET subject_template = EXCLUDED.subject_template,
		    text_template = EXCLUDED.text_template,
		    html_template = EXCLUDED.html_template,
		    updated_by_user_id = EXCLUDED.updated_by_user_id,
		    revision = email_template_overrides.revision + 1
		RETURNING `+emailTemplateOverrideColumns,
		string(in.Template),
		in.SubjectTemplate,
		in.TextTemplate,
		in.HTMLTemplate,
		in.UpdatedByUserID,
	), &row); err != nil {
		return domain.EmailTemplateOverride{}, err
	}
	return toDomainEmailTemplateOverride(row), nil
}

func (s *Store) DeleteEmailTemplateOverride(ctx context.Context, key domain.EmailTemplate) error {
	result, err := s.exec(ctx, `
		DELETE FROM email_template_overrides
		WHERE template_key = $1
	`, string(key))
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrEmailTemplateOverrideNotFound
	}
	return nil
}
