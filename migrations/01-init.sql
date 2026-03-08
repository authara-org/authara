-- +migrate Up
CREATE SCHEMA IF NOT EXISTS authara;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- +migrate StatementBegin
CREATE OR REPLACE FUNCTION authara.set_updated_at()
RETURNS trigger AS $func$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$func$ LANGUAGE plpgsql;
-- +migrate StatementEnd

CREATE TABLE IF NOT EXISTS public.schema_version (
  version INTEGER NOT NULL PRIMARY KEY
);

INSERT INTO schema_version (version)
VALUES (1)
ON CONFLICT (version) DO NOTHING;

CREATE TABLE IF NOT EXISTS authara.users (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	username varchar(255) NOT NULL,
	username_normalized varchar(255) NOT NULL,
	email varchar(255) NOT NULL,
	disabled_at timestamptz,

	CONSTRAINT unique_user_email UNIQUE (email),
	CONSTRAINT unuique_user_username UNIQUE (username)
);

DROP TRIGGER IF EXISTS trg_user_updated_at ON authara.users;
CREATE TRIGGER trg_user_updated_at
BEFORE UPDATE ON authara.users
FOR EACH ROW
EXECUTE FUNCTION authara.set_updated_at();


CREATE TABLE IF NOT EXISTS authara.roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO authara.roles (name)
VALUES ('admin')
ON CONFLICT (name) DO NOTHING;


CREATE TABLE IF NOT EXISTS authara.user_roles (
	user_id UUID NOT NULL REFERENCES authara.users(id) ON DELETE CASCADE,
	role_id UUID NOT NULL REFERENCES authara.roles(id) ON DELETE CASCADE,
	PRIMARY KEY (user_id, role_id)
);


CREATE TABLE IF NOT EXISTS authara.auth_providers (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	user_id uuid NOT NULL REFERENCES authara.users(id) ON DELETE CASCADE,
	provider varchar(50) NOT NULL,
	provider_user_id varchar(255),
	password_hash varchar(255),
	two_factor_authentication boolean NOT NULL DEFAULT false,
	
	CONSTRAINT unique_user_provider UNIQUE (user_id, provider)
);

CREATE UNIQUE INDEX IF NOT EXISTS unique_provider_user_nonnull
ON authara.auth_providers (provider, provider_user_id)
WHERE provider_user_id IS NOT NULL;

DROP TRIGGER IF EXISTS trg_auth_provider_updated_at ON authara.auth_providers;
CREATE TRIGGER trg_auth_provider_updated_at
BEFORE UPDATE ON authara.auth_providers
FOR EACH ROW
EXECUTE FUNCTION authara.set_updated_at();


CREATE TABLE IF NOT EXISTS authara.sessions (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	user_id uuid NOT NULL REFERENCES authara.users(id) ON DELETE CASCADE,
	expires_at timestamptz NOT NULL,
	revoked_at timestamptz,
	user_agent varchar(255) NOT NULL
);

DROP TRIGGER IF EXISTS trg_session_updated_at ON authara.sessions;
CREATE TRIGGER trg_session_updated_at
BEFORE UPDATE ON authara.sessions
FOR EACH ROW
EXECUTE FUNCTION authara.set_updated_at();


CREATE TABLE IF NOT EXISTS authara.refresh_tokens (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	created_at timestamptz NOT NULL DEFAULT now(),
	session_id uuid NOT NULL REFERENCES authara.sessions(id) ON DELETE CASCADE,
	token_hash varchar(512) NOT NULL UNIQUE,
	expires_at timestamptz NOT NULL,
	consumed_at timestamptz
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_session_id
ON authara.refresh_tokens(session_id);


-- +migrate Down
DROP SCHEMA authara CASCADE;
