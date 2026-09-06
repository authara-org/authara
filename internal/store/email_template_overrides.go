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

const emailTemplateVersionColumns = `
	template_key,
	version,
	created_at,
	subject_template,
	text_template,
	html_template,
	created_by_user_id
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

func scanEmailTemplateVersion(row rowScanner, m *model.EmailTemplateVersion) error {
	return row.Scan(
		&m.TemplateKey,
		&m.Version,
		&m.CreatedAt,
		&m.SubjectTemplate,
		&m.TextTemplate,
		&m.HTMLTemplate,
		&m.CreatedByUserID,
	)
}

func toDomainEmailTemplateVersion(m model.EmailTemplateVersion) domain.EmailTemplateVersion {
	return domain.EmailTemplateVersion{
		Template:        domain.EmailTemplate(m.TemplateKey),
		Version:         m.Version,
		CreatedAt:       m.CreatedAt,
		SubjectTemplate: m.SubjectTemplate,
		TextTemplate:    m.TextTemplate,
		HTMLTemplate:    m.HTMLTemplate,
		CreatedByUserID: m.CreatedByUserID,
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

func (s *Store) GetEmailTemplateVersion(ctx context.Context, key domain.EmailTemplate, version int64) (domain.EmailTemplateVersion, error) {
	var row model.EmailTemplateVersion
	if err := scanEmailTemplateVersion(s.queryRow(ctx, `
		SELECT `+emailTemplateVersionColumns+`
		FROM email_template_versions
		WHERE template_key = $1
		  AND version = $2
	`, string(key), version), &row); err != nil {
		return domain.EmailTemplateVersion{}, mapNoRows(err, ErrEmailTemplateVersionNotFound)
	}
	return toDomainEmailTemplateVersion(row), nil
}

func (s *Store) ListEmailTemplateVersions(ctx context.Context, key domain.EmailTemplate) ([]domain.EmailTemplateVersion, error) {
	rows, err := s.queryRows(ctx, `
		SELECT `+emailTemplateVersionColumns+`
		FROM email_template_versions
		WHERE template_key = $1
		ORDER BY version DESC
	`, string(key))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.EmailTemplateVersion, 0)
	for rows.Next() {
		var row model.EmailTemplateVersion
		if err := scanEmailTemplateVersion(rows, &row); err != nil {
			return nil, err
		}
		out = append(out, toDomainEmailTemplateVersion(row))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) UpsertEmailTemplateOverride(ctx context.Context, in domain.EmailTemplateOverride, expectedRevision int64) (domain.EmailTemplateOverride, error) {
	var row model.EmailTemplateOverride
	if err := scanEmailTemplateOverride(s.queryRow(ctx, `
		WITH updated AS (
			UPDATE email_template_overrides
			SET subject_template = $2,
			    text_template = $3,
			    html_template = $4,
			    updated_by_user_id = $5,
			    revision = revision + 1
			WHERE template_key = $1
			  AND revision = $6
			RETURNING `+emailTemplateOverrideColumns+`
		), inserted AS (
			INSERT INTO email_template_overrides (
				template_key,
				subject_template,
				text_template,
				html_template,
				updated_by_user_id
			)
			SELECT $1, $2, $3, $4, $5
			WHERE $6 = 0
			  AND NOT EXISTS (SELECT 1 FROM updated)
			ON CONFLICT (template_key) DO NOTHING
			RETURNING `+emailTemplateOverrideColumns+`
		), saved AS (
			SELECT `+emailTemplateOverrideColumns+` FROM updated
			UNION ALL
			SELECT `+emailTemplateOverrideColumns+` FROM inserted
		), recorded AS (
			INSERT INTO email_template_versions (
				template_key,
				version,
				subject_template,
				text_template,
				html_template,
				created_by_user_id
			)
			SELECT
				saved.template_key,
				COALESCE((
					SELECT max(history.version)
					FROM email_template_versions AS history
					WHERE history.template_key = saved.template_key
				), 0) + 1,
				saved.subject_template,
				saved.text_template,
				saved.html_template,
				saved.updated_by_user_id
			FROM saved
			RETURNING template_key
		)
		SELECT `+emailTemplateOverrideColumns+`
		FROM saved
		WHERE EXISTS (SELECT 1 FROM recorded)
	`,
		string(in.Template),
		in.SubjectTemplate,
		in.TextTemplate,
		in.HTMLTemplate,
		in.UpdatedByUserID,
		expectedRevision,
	), &row); err != nil {
		return domain.EmailTemplateOverride{}, mapNoRows(err, ErrEmailTemplateRevisionConflict)
	}
	return toDomainEmailTemplateOverride(row), nil
}

func (s *Store) DeleteEmailTemplateOverride(ctx context.Context, key domain.EmailTemplate, expectedRevision int64) error {
	result, err := s.exec(ctx, `
		DELETE FROM email_template_overrides
		WHERE template_key = $1
		  AND revision = $2
	`, string(key), expectedRevision)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrEmailTemplateRevisionConflict
	}
	return nil
}
