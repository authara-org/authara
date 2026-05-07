-- +migrate Up

CREATE TABLE IF NOT EXISTS authara.allowed_emails (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	email varchar(255) NOT NULL,

	CONSTRAINT unique_allowed_email UNIQUE (email)
);

-- +migrate Down

DROP TABLE IF EXISTS authara.allowed_emails;
