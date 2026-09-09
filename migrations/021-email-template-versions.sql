-- +migrate Up

CREATE TABLE IF NOT EXISTS authara.email_template_versions (
	template_key varchar(100) NOT NULL,
	version bigint NOT NULL,
	created_at timestamptz NOT NULL DEFAULT now(),
	subject_template text NOT NULL,
	text_template text NOT NULL,
	html_template text NOT NULL,
	created_by_user_id uuid REFERENCES authara.users(id) ON DELETE SET NULL,

	PRIMARY KEY (template_key, version),
	CONSTRAINT email_template_version_subject_not_blank CHECK (btrim(subject_template) <> ''),
	CONSTRAINT email_template_version_text_not_blank CHECK (btrim(text_template) <> ''),
	CONSTRAINT email_template_version_html_not_blank CHECK (btrim(html_template) <> ''),
	CONSTRAINT email_template_version_positive CHECK (version > 0)
);

INSERT INTO authara.email_template_versions (
	template_key,
	version,
	created_at,
	subject_template,
	text_template,
	html_template,
	created_by_user_id
)
SELECT
	template_key,
	revision,
	updated_at,
	subject_template,
	text_template,
	html_template,
	updated_by_user_id
FROM authara.email_template_overrides
ON CONFLICT (template_key, version) DO NOTHING;

INSERT INTO public.authara_schema_version (version)
VALUES (21)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DELETE FROM public.authara_schema_version
WHERE version = 21;

DROP TABLE IF EXISTS authara.email_template_versions;
