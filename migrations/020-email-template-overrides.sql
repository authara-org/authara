-- +migrate Up

CREATE TABLE IF NOT EXISTS authara.email_template_overrides (
	template_key varchar(100) PRIMARY KEY,
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	subject_template text NOT NULL,
	text_template text NOT NULL,
	html_template text NOT NULL,
	revision bigint NOT NULL DEFAULT 1,
	updated_by_user_id uuid REFERENCES authara.users(id) ON DELETE SET NULL,

	CONSTRAINT email_template_override_subject_not_blank CHECK (btrim(subject_template) <> ''),
	CONSTRAINT email_template_override_text_not_blank CHECK (btrim(text_template) <> ''),
	CONSTRAINT email_template_override_html_not_blank CHECK (btrim(html_template) <> ''),
	CONSTRAINT email_template_override_revision_positive CHECK (revision > 0)
);

DROP TRIGGER IF EXISTS trg_email_template_override_updated_at ON authara.email_template_overrides;
CREATE TRIGGER trg_email_template_override_updated_at
BEFORE UPDATE ON authara.email_template_overrides
FOR EACH ROW
EXECUTE FUNCTION authara.set_updated_at();

INSERT INTO public.authara_schema_version (version)
VALUES (20)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DELETE FROM public.authara_schema_version
WHERE version = 20;

DROP TRIGGER IF EXISTS trg_email_template_override_updated_at ON authara.email_template_overrides;
DROP TABLE IF EXISTS authara.email_template_overrides;
