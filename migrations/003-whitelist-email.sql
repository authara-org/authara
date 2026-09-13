-- +migrate Up

CREATE TABLE IF NOT EXISTS authara.allowed_emails (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	email varchar(255) NOT NULL,

	CONSTRAINT unique_allowed_email UNIQUE (email)
);

INSERT INTO public.authara_schema_version (version)
VALUES (3)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DELETE FROM public.authara_schema_version
WHERE version = 3;

DROP TABLE IF EXISTS authara.allowed_emails;
