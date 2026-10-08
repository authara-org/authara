-- +migrate Up

CREATE TABLE authara.authentication_challenges (
	id uuid PRIMARY KEY,
	created_at timestamptz NOT NULL,
	user_id uuid NOT NULL REFERENCES authara.users(id) ON DELETE CASCADE,
	session_id uuid NOT NULL REFERENCES authara.sessions(id) ON DELETE CASCADE,
	expires_at timestamptz NOT NULL,
	consumed_at timestamptz,
	authentication_method varchar(32),

	CONSTRAINT unique_authentication_challenge_session UNIQUE (session_id),
	CONSTRAINT chk_authentication_challenge_completion CHECK (
		(consumed_at IS NULL AND authentication_method IS NULL)
		OR
		(consumed_at IS NOT NULL AND authentication_method IN ('password', 'passkey', 'google'))
	)
);

CREATE INDEX idx_authentication_challenges_expires_at
ON authara.authentication_challenges (expires_at);

INSERT INTO public.authara_schema_version (version)
VALUES (28)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DELETE FROM public.authara_schema_version
WHERE version = 28;

DROP INDEX IF EXISTS authara.idx_authentication_challenges_expires_at;
DROP TABLE IF EXISTS authara.authentication_challenges;
