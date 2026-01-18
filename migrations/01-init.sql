-- +migrate Up
CREATE SCHEMA IF NOT EXISTS authgate;


-- UUID generation (Postgres 13+)
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Auto-update updated_at
-- +migrate StatementBegin
CREATE OR REPLACE FUNCTION authgate.set_updated_at()
RETURNS trigger AS $func$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$func$ LANGUAGE plpgsql;
-- +migrate StatementEnd



CREATE TABLE IF NOT EXISTS authgate.users (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),

	username varchar(255) NOT NULL,
	email varchar(255) NOT NULL,

	CONSTRAINT unique_user_email UNIQUE (email)
);

CREATE TRIGGER trg_user_updated_at
BEFORE UPDATE ON authgate.users
FOR EACH ROW
EXECUTE FUNCTION authgate.set_updated_at();


CREATE TABLE IF NOT EXISTS authgate.auth_providers (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),

	user_id uuid NOT NULL REFERENCES authgate.users(id) ON DELETE CASCADE,
	provider varchar(50) NOT NULL,
	provider_user_id varchar(255),
	password_hash varchar(255),
	two_factor_authentication boolean NOT NULL DEFAULT false,

	CONSTRAINT unique_provider_user UNIQUE (provider, provider_user_id)
);

CREATE TRIGGER trg_auth_provider_updated_at
BEFORE UPDATE ON authgate.auth_providers
FOR EACH ROW
EXECUTE FUNCTION authgate.set_updated_at();


CREATE TABLE IF NOT EXISTS authgate.sessions (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),

	user_id uuid NOT NULL REFERENCES authgate.users(id) ON DELETE CASCADE,

	refresh_token varchar(512) NOT NULL UNIQUE,
	issued_at timestamptz NOT NULL DEFAULT now(),
	expires_at timestamptz NOT NULL,
	revoked boolean NOT NULL DEFAULT false,

	user_agent varchar(255)
);

CREATE TRIGGER trg_session_updated_at
BEFORE UPDATE ON authgate.sessions
FOR EACH ROW
EXECUTE FUNCTION authgate.set_updated_at();

-- +migrate Down
DROP SCHEMA authgate CASCADE;
