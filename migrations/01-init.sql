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


CREATE TABLE roles (
    id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name      TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);


CREATE TABLE authgate.user_roles (
	user_id UUID NOT NULL REFERENCES authgate.users(id) ON DELETE CASCADE,
	role_id UUID NOT NULL REFERENCES authgate.roles(id) ON DELETE CASCADE,
	PRIMARY KEY (user_id, role_id)
);

INSERT INTO authgate.roles (name)
VALUES ('authgate:admin')
ON CONFLICT (name) DO NOTHING;


CREATE TABLE IF NOT EXISTS authgate.auth_providers (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),

	user_id uuid NOT NULL REFERENCES authgate.users(id) ON DELETE CASCADE,
	provider varchar(50) NOT NULL,
	provider_user_id varchar(255),
	password_hash varchar(255),
	two_factor_authentication boolean NOT NULL DEFAULT false,

	CONSTRAINT unique_user_provider UNIQUE (user_id, provider)
);

CREATE UNIQUE INDEX unique_provider_user_nonnull
ON authgate.auth_providers (provider, provider_user_id)
WHERE provider_user_id IS NOT NULL;

CREATE TRIGGER trg_auth_provider_updated_at
BEFORE UPDATE ON authgate.auth_providers
FOR EACH ROW
EXECUTE FUNCTION authgate.set_updated_at();


CREATE TABLE IF NOT EXISTS authgate.sessions (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),

	user_id uuid NOT NULL REFERENCES authgate.users(id) ON DELETE CASCADE,

	expires_at timestamptz NOT NULL,
	revoked_at timestamptz,

	user_agent varchar(255) NOT NULL
);

CREATE TRIGGER trg_session_updated_at
BEFORE UPDATE ON authgate.sessions
FOR EACH ROW
EXECUTE FUNCTION authgate.set_updated_at();


CREATE TABLE IF NOT EXISTS authgate.refresh_tokens (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	created_at timestamptz NOT NULL DEFAULT now(),

	session_id uuid NOT NULL REFERENCES authgate.sessions(id) ON DELETE CASCADE,
	token_hash varchar(512) NOT NULL UNIQUE,

	expires_at timestamptz NOT NULL,
	consumed_at timestamptz
);

CREATE INDEX idx_refresh_tokens_session_id
ON authgate.refresh_tokens(session_id);

-- +migrate Down
DROP SCHEMA authgate CASCADE;
