-- +migrate Up

CREATE TABLE IF NOT EXISTS authara.passkeys (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	user_id uuid NOT NULL REFERENCES authara.users(id) ON DELETE CASCADE,
	credential_id bytea NOT NULL,
	public_key bytea NOT NULL,
	attestation_type text NOT NULL DEFAULT '',
	attestation_format text NOT NULL DEFAULT '',
	transport text[] NOT NULL DEFAULT '{}',
	aaguid uuid,
	sign_count bigint NOT NULL DEFAULT 0,
	clone_warning boolean NOT NULL DEFAULT false,
	name varchar(255) NOT NULL DEFAULT 'Passkey',
	last_used_at timestamptz,
	user_present boolean NOT NULL DEFAULT false,
	user_verified boolean NOT NULL DEFAULT false,
	backup_eligible boolean NOT NULL DEFAULT false,
	backup_state boolean NOT NULL DEFAULT false,

	CONSTRAINT unique_passkey_credential_id UNIQUE (credential_id)
);

CREATE INDEX IF NOT EXISTS idx_passkeys_user_id
ON authara.passkeys (user_id);

DROP TRIGGER IF EXISTS trg_passkey_updated_at ON authara.passkeys;
CREATE TRIGGER trg_passkey_updated_at
BEFORE UPDATE ON authara.passkeys
FOR EACH ROW
EXECUTE FUNCTION authara.set_updated_at();

CREATE TABLE IF NOT EXISTS authara.webauthn_challenges (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	created_at timestamptz NOT NULL DEFAULT now(),
	user_id uuid REFERENCES authara.users(id) ON DELETE CASCADE,
	purpose varchar(64) NOT NULL,
	challenge text NOT NULL,
	session_data jsonb NOT NULL,
	expires_at timestamptz NOT NULL,
	consumed_at timestamptz,

	CONSTRAINT chk_webauthn_challenges_purpose
	CHECK (purpose IN ('registration', 'authentication'))
);

CREATE INDEX IF NOT EXISTS idx_webauthn_challenges_expires_at
ON authara.webauthn_challenges (expires_at);

INSERT INTO public.authara_schema_version (version)
VALUES (10)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DELETE FROM public.authara_schema_version
WHERE version = 10;

DROP INDEX IF EXISTS authara.idx_webauthn_challenges_expires_at;
DROP TABLE IF EXISTS authara.webauthn_challenges;

DROP TRIGGER IF EXISTS trg_passkey_updated_at ON authara.passkeys;
DROP INDEX IF EXISTS authara.idx_passkeys_user_id;
DROP TABLE IF EXISTS authara.passkeys;
