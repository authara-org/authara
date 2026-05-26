-- +migrate Up

CREATE TABLE IF NOT EXISTS authara.admin_audit_events (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	created_at timestamptz NOT NULL DEFAULT now(),
	actor_user_id uuid REFERENCES authara.users(id) ON DELETE SET NULL,
	action varchar(100) NOT NULL,
	target_user_id uuid REFERENCES authara.users(id) ON DELETE SET NULL,
	target_email varchar(255),
	metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
	ip varchar(255),
	user_agent varchar(512)
);

CREATE INDEX IF NOT EXISTS idx_admin_audit_events_created_at
ON authara.admin_audit_events (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_admin_audit_events_actor_user_id
ON authara.admin_audit_events (actor_user_id);

CREATE INDEX IF NOT EXISTS idx_admin_audit_events_target_user_id
ON authara.admin_audit_events (target_user_id);

CREATE INDEX IF NOT EXISTS idx_admin_audit_events_action
ON authara.admin_audit_events (action);

CREATE INDEX IF NOT EXISTS idx_admin_audit_events_target_email
ON authara.admin_audit_events (target_email);

INSERT INTO public.authara_schema_version (version)
VALUES (11)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DELETE FROM public.authara_schema_version
WHERE version = 11;

DROP INDEX IF EXISTS authara.idx_admin_audit_events_target_email;
DROP INDEX IF EXISTS authara.idx_admin_audit_events_action;
DROP INDEX IF EXISTS authara.idx_admin_audit_events_target_user_id;
DROP INDEX IF EXISTS authara.idx_admin_audit_events_actor_user_id;
DROP INDEX IF EXISTS authara.idx_admin_audit_events_created_at;
DROP TABLE IF EXISTS authara.admin_audit_events;
