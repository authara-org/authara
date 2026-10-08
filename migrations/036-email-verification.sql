-- +migrate Up

ALTER TABLE authara.users
ADD COLUMN email_verified_at timestamptz;

CREATE TABLE authara.email_verification_transactions (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	created_at timestamptz NOT NULL DEFAULT now(),
	user_id uuid NOT NULL REFERENCES authara.users(id) ON DELETE CASCADE,
	original_session_id uuid NOT NULL REFERENCES authara.sessions(id) ON DELETE CASCADE,
	audience varchar(32) NOT NULL,
	authentication_method varchar(32) NOT NULL,
	return_to varchar(2048) NOT NULL,
	expires_at timestamptz NOT NULL,
	consumed_at timestamptz,
	challenge_id uuid REFERENCES authara.challenges(id) ON DELETE SET NULL,
	target_email varchar(255),

	CONSTRAINT unique_email_verification_transaction_user UNIQUE (user_id),
	CONSTRAINT unique_email_verification_transaction_challenge UNIQUE (challenge_id),
	CONSTRAINT email_verification_transaction_audience_check
		CHECK (audience IN ('app', 'admin', 'operator')),
	CONSTRAINT email_verification_transaction_authentication_method_check
		CHECK (authentication_method IN ('password', 'passkey', 'google')),
	CONSTRAINT email_verification_transaction_target_check CHECK (
		(challenge_id IS NULL AND target_email IS NULL)
		OR (challenge_id IS NOT NULL AND target_email IS NOT NULL)
	)
);

CREATE INDEX idx_email_verification_transactions_expires_at
ON authara.email_verification_transactions (expires_at);

INSERT INTO public.authara_schema_version (version)
VALUES (36)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DELETE FROM public.authara_schema_version
WHERE version = 36;

DROP INDEX IF EXISTS authara.idx_email_verification_transactions_expires_at;
DROP TABLE IF EXISTS authara.email_verification_transactions;

ALTER TABLE authara.users
DROP COLUMN IF EXISTS email_verified_at;
