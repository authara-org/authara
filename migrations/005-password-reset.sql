-- +migrate Up

CREATE TABLE IF NOT EXISTS authara.pending_password_resets (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	created_at timestamptz NOT NULL DEFAULT now(),

	challenge_id uuid NOT NULL REFERENCES authara.challenges(id) ON DELETE CASCADE,
	user_id uuid NOT NULL REFERENCES authara.users(id) ON DELETE CASCADE,
	password_hash varchar(255) NOT NULL,

	CONSTRAINT unique_pending_password_reset_challenge UNIQUE (challenge_id)
);

CREATE INDEX IF NOT EXISTS idx_pending_password_resets_user_id
ON authara.pending_password_resets (user_id);

-- +migrate Down

DROP INDEX IF EXISTS authara.idx_pending_password_resets_user_id;
DROP TABLE IF EXISTS authara.pending_password_resets;
