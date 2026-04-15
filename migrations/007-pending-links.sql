-- +migrate Up

CREATE TABLE IF NOT EXISTS authara.pending_provider_links (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	created_at timestamptz NOT NULL DEFAULT now(),

	user_id uuid NOT NULL REFERENCES authara.users(id) ON DELETE CASCADE,
	session_id uuid NOT NULL REFERENCES authara.sessions(id) ON DELETE CASCADE,
	provider varchar(50) NOT NULL,

	expires_at timestamptz NOT NULL,
	consumed_at timestamptz
);

CREATE INDEX IF NOT EXISTS idx_pending_provider_links_user_id
ON authara.pending_provider_links (user_id);

CREATE INDEX IF NOT EXISTS idx_pending_provider_links_session_id
ON authara.pending_provider_links (session_id);

-- +migrate Down

DROP INDEX IF EXISTS authara.idx_pending_provider_links_session_id;
DROP INDEX IF EXISTS authara.idx_pending_provider_links_user_id;
DROP TABLE IF EXISTS authara.pending_provider_links;
