package store

import (
	"context"

	"github.com/authara-org/authara/internal/domain"
	"github.com/authara-org/authara/internal/store/model"
	"github.com/google/uuid"
)

const emailTemplateDeliverySettingColumns = `
	template_key,
	created_at,
	updated_at,
	enabled,
	updated_by_user_id
`

func scanEmailTemplateDeliverySetting(row rowScanner, setting *model.EmailTemplateDeliverySetting) error {
	return row.Scan(
		&setting.TemplateKey,
		&setting.CreatedAt,
		&setting.UpdatedAt,
		&setting.Enabled,
		&setting.UpdatedByUserID,
	)
}

func toDomainEmailTemplateDeliverySetting(setting model.EmailTemplateDeliverySetting) domain.EmailTemplateDeliverySetting {
	return domain.EmailTemplateDeliverySetting{
		Template:        domain.EmailTemplate(setting.TemplateKey),
		Enabled:         setting.Enabled,
		CreatedAt:       setting.CreatedAt,
		UpdatedAt:       setting.UpdatedAt,
		UpdatedByUserID: setting.UpdatedByUserID,
	}
}

func (s *Store) ListEmailTemplateDeliverySettings(ctx context.Context) ([]domain.EmailTemplateDeliverySetting, error) {
	rows, err := s.queryRows(ctx, `
		SELECT `+emailTemplateDeliverySettingColumns+`
		FROM email_template_delivery_settings
		ORDER BY template_key ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := make([]domain.EmailTemplateDeliverySetting, 0)
	for rows.Next() {
		var setting model.EmailTemplateDeliverySetting
		if err := scanEmailTemplateDeliverySetting(rows, &setting); err != nil {
			return nil, err
		}
		settings = append(settings, toDomainEmailTemplateDeliverySetting(setting))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return settings, nil
}

func (s *Store) SetEmailTemplateDeliveryEnabled(
	ctx context.Context,
	key domain.EmailTemplate,
	enabled bool,
	actorUserID uuid.UUID,
) (domain.EmailTemplateDeliverySetting, error) {
	action := domain.OperatorAuditActionEmailTemplateDeliveryDisabled
	if enabled {
		action = domain.OperatorAuditActionEmailTemplateDeliveryEnabled
	}

	var setting model.EmailTemplateDeliverySetting
	err := scanEmailTemplateDeliverySetting(s.queryRow(ctx, `
		WITH saved AS (
			INSERT INTO email_template_delivery_settings (
				template_key,
				enabled,
				updated_by_user_id
			)
			VALUES ($1, $2, $3)
			ON CONFLICT (template_key) DO UPDATE
			SET enabled = EXCLUDED.enabled,
			    updated_by_user_id = EXCLUDED.updated_by_user_id
			RETURNING `+emailTemplateDeliverySettingColumns+`
		), audited AS (
			INSERT INTO operator_audit_events (
				actor_user_id,
				action,
				resource_type,
				resource_id,
				metadata
			)
			SELECT
				$3,
				$4,
				$5,
				saved.template_key,
				jsonb_build_object('enabled', saved.enabled)
			FROM saved
			RETURNING id
		)
		SELECT `+emailTemplateDeliverySettingColumns+`
		FROM saved
		WHERE EXISTS (SELECT 1 FROM audited)
	`,
		string(key),
		enabled,
		actorUserID,
		action,
		domain.OperatorAuditResourceEmailTemplate,
	), &setting)
	if err != nil {
		return domain.EmailTemplateDeliverySetting{}, err
	}
	return toDomainEmailTemplateDeliverySetting(setting), nil
}
