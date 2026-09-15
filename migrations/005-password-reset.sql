-- +migrate Up

CREATE TABLE IF NOT EXISTS authara.pending_password_resets (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	created_at timestamptz NOT NULL DEFAULT now(),

	challenge_id uuid NOT NULL REFERENCES authara.challenges(id) ON DELETE CASCADE,
	user_id uuid NOT NULL REFERENCES authara.users(id) ON DELETE CASCADE,
	password_hash varchar(255) NOT NULL,

	CONSTRAINT unique_pending_password_reset_challenge UNIQUE (challenge_id),
	CONSTRAINT unique_pending_password_reset_user UNIQUE (user_id)
);

INSERT INTO public.authara_schema_version (version)
VALUES (5)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DELETE FROM public.authara_schema_version
WHERE version = 5;

DROP TABLE IF EXISTS authara.pending_password_resets;
