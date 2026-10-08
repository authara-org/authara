-- +migrate Up

ALTER TABLE authara.sessions
ADD COLUMN authenticated_at timestamptz,
ADD COLUMN authentication_method varchar(32),
ADD CONSTRAINT chk_sessions_authentication_provenance
CHECK (
	(authenticated_at IS NULL AND authentication_method IS NULL)
	OR
	(authenticated_at IS NOT NULL AND authentication_method IN ('password', 'passkey', 'google'))
);

ALTER TABLE authara.webauthn_challenges
ADD COLUMN session_id uuid REFERENCES authara.sessions(id) ON DELETE CASCADE;

ALTER TABLE authara.webauthn_challenges
DROP CONSTRAINT chk_webauthn_challenges_purpose,
ADD CONSTRAINT chk_webauthn_challenges_purpose
CHECK (purpose IN ('registration', 'authentication', 'reauthentication'));

INSERT INTO public.authara_schema_version (version)
VALUES (27)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DELETE FROM public.authara_schema_version
WHERE version = 27;

DELETE FROM authara.webauthn_challenges
WHERE purpose = 'reauthentication';

ALTER TABLE authara.webauthn_challenges
DROP CONSTRAINT chk_webauthn_challenges_purpose,
ADD CONSTRAINT chk_webauthn_challenges_purpose
CHECK (purpose IN ('registration', 'authentication')),
DROP COLUMN session_id;

ALTER TABLE authara.sessions
DROP CONSTRAINT chk_sessions_authentication_provenance,
DROP COLUMN authentication_method,
DROP COLUMN authenticated_at;
