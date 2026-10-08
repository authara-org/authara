-- +migrate Up

ALTER TABLE authara.sessions
DROP CONSTRAINT chk_sessions_authentication_provenance,
ADD CONSTRAINT chk_sessions_authentication_provenance
CHECK (
	(authenticated_at IS NULL AND authentication_method IS NULL)
	OR
	(authenticated_at IS NOT NULL AND authentication_method IN ('password', 'passkey', 'google', 'apple'))
);

ALTER TABLE authara.authentication_challenges
DROP CONSTRAINT chk_authentication_challenge_completion,
ADD CONSTRAINT chk_authentication_challenge_completion CHECK (
	(consumed_at IS NULL AND authentication_method IS NULL)
	OR
	(consumed_at IS NOT NULL AND authentication_method IN ('password', 'passkey', 'google', 'apple'))
);

ALTER TABLE authara.security_events
DROP CONSTRAINT security_events_authentication_method_check,
ADD CONSTRAINT security_events_authentication_method_check
CHECK (authentication_method IS NULL OR authentication_method IN ('password', 'passkey', 'google', 'apple')),
DROP CONSTRAINT security_events_response_check,
ADD CONSTRAINT security_events_response_check
CHECK (response IS NULL OR response IN ('password', 'google', 'apple', 'alert', 'restrict', 'restrict_and_revoke'));

ALTER TABLE authara.email_verification_transactions
DROP CONSTRAINT email_verification_transaction_authentication_method_check,
ADD CONSTRAINT email_verification_transaction_authentication_method_check
CHECK (authentication_method IN ('password', 'passkey', 'google', 'apple'));

CREATE TABLE authara.apple_credentials (
	auth_provider_id uuid PRIMARY KEY REFERENCES authara.auth_providers(id) ON DELETE CASCADE,
	encryption_key_id varchar(128) NOT NULL,
	encrypted_refresh_token bytea NOT NULL,
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now()
);

DROP TRIGGER IF EXISTS trg_apple_credential_updated_at ON authara.apple_credentials;
CREATE TRIGGER trg_apple_credential_updated_at
BEFORE UPDATE ON authara.apple_credentials
FOR EACH ROW
EXECUTE FUNCTION authara.set_updated_at();

-- Revocation work must survive deletion of the user and auth-provider rows.
-- Tokens stay encrypted with the same keyring as apple_credentials; the
-- encryption_context column records the AES-GCM additional data needed to
-- decrypt either an archived credential or a newly returned unused token.
CREATE TABLE authara.apple_token_revocations (
	id uuid PRIMARY KEY,
	auth_provider_id uuid,
	encryption_context uuid NOT NULL,
	encryption_key_id varchar(128) NOT NULL,
	encrypted_refresh_token bytea NOT NULL,
	attempt_count integer NOT NULL DEFAULT 0,
	next_attempt_at timestamptz NOT NULL DEFAULT now(),
	last_error text,
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now()
);

DROP TRIGGER IF EXISTS trg_apple_token_revocation_updated_at ON authara.apple_token_revocations;
CREATE TRIGGER trg_apple_token_revocation_updated_at
BEFORE UPDATE ON authara.apple_token_revocations
FOR EACH ROW
EXECUTE FUNCTION authara.set_updated_at();

CREATE INDEX idx_apple_token_revocations_due
ON authara.apple_token_revocations (next_attempt_at, created_at, id);

INSERT INTO public.authara_schema_version (version)
VALUES (37)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DELETE FROM public.authara_schema_version
WHERE version = 37;

DROP TABLE IF EXISTS authara.apple_credentials;

ALTER TABLE authara.email_verification_transactions
DROP CONSTRAINT email_verification_transaction_authentication_method_check,
ADD CONSTRAINT email_verification_transaction_authentication_method_check
CHECK (authentication_method IN ('password', 'passkey', 'google'));

ALTER TABLE authara.security_events
DROP CONSTRAINT security_events_response_check,
ADD CONSTRAINT security_events_response_check
CHECK (response IS NULL OR response IN ('password', 'google', 'alert', 'restrict', 'restrict_and_revoke')),
DROP CONSTRAINT security_events_authentication_method_check,
ADD CONSTRAINT security_events_authentication_method_check
CHECK (authentication_method IS NULL OR authentication_method IN ('password', 'passkey', 'google'));

ALTER TABLE authara.authentication_challenges
DROP CONSTRAINT chk_authentication_challenge_completion,
ADD CONSTRAINT chk_authentication_challenge_completion CHECK (
	(consumed_at IS NULL AND authentication_method IS NULL)
	OR
	(consumed_at IS NOT NULL AND authentication_method IN ('password', 'passkey', 'google'))
);

ALTER TABLE authara.sessions
DROP CONSTRAINT chk_sessions_authentication_provenance,
ADD CONSTRAINT chk_sessions_authentication_provenance
CHECK (
	(authenticated_at IS NULL AND authentication_method IS NULL)
	OR
	(authenticated_at IS NOT NULL AND authentication_method IN ('password', 'passkey', 'google'))
);
