-- +migrate Up

ALTER TABLE authara.passkeys
ADD COLUMN restricted_at timestamptz;

CREATE TABLE authara.security_events (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	created_at timestamptz NOT NULL DEFAULT now(),
	type varchar(100) NOT NULL,
	user_id uuid REFERENCES authara.users(id) ON DELETE SET NULL,
	passkey_id uuid REFERENCES authara.passkeys(id) ON DELETE SET NULL,
	response varchar(64) NOT NULL
);

CREATE INDEX idx_security_events_created_at
ON authara.security_events (created_at DESC);

CREATE INDEX idx_security_events_user_id
ON authara.security_events (user_id);

INSERT INTO public.authara_schema_version (version)
VALUES (29)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DELETE FROM public.authara_schema_version
WHERE version = 29;

DROP INDEX IF EXISTS authara.idx_security_events_user_id;
DROP INDEX IF EXISTS authara.idx_security_events_created_at;
DROP TABLE IF EXISTS authara.security_events;

ALTER TABLE authara.passkeys
DROP COLUMN IF EXISTS restricted_at;
