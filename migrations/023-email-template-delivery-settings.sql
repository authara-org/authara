-- +migrate Up

CREATE TABLE IF NOT EXISTS authara.email_template_delivery_settings (
	template_key varchar(100) PRIMARY KEY,
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	enabled boolean NOT NULL DEFAULT true,
	updated_by_user_id uuid REFERENCES authara.users(id) ON DELETE SET NULL
);

DROP TRIGGER IF EXISTS trg_email_template_delivery_setting_updated_at ON authara.email_template_delivery_settings;
CREATE TRIGGER trg_email_template_delivery_setting_updated_at
BEFORE UPDATE ON authara.email_template_delivery_settings
FOR EACH ROW
EXECUTE FUNCTION authara.set_updated_at();

INSERT INTO public.authara_schema_version (version)
VALUES (23)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DELETE FROM public.authara_schema_version
WHERE version = 23;

DROP TRIGGER IF EXISTS trg_email_template_delivery_setting_updated_at ON authara.email_template_delivery_settings;
DROP TABLE IF EXISTS authara.email_template_delivery_settings;
