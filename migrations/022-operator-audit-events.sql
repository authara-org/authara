-- +migrate Up

CREATE TABLE IF NOT EXISTS authara.operator_audit_events (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	created_at timestamptz NOT NULL DEFAULT now(),
	actor_user_id uuid REFERENCES authara.users(id) ON DELETE SET NULL,
	action varchar(100) NOT NULL,
	resource_type varchar(100) NOT NULL,
	resource_id varchar(255) NOT NULL,
	metadata jsonb NOT NULL DEFAULT '{}'::jsonb,

	CONSTRAINT operator_audit_event_action_not_blank CHECK (btrim(action) <> ''),
	CONSTRAINT operator_audit_event_resource_type_not_blank CHECK (btrim(resource_type) <> ''),
	CONSTRAINT operator_audit_event_resource_id_not_blank CHECK (btrim(resource_id) <> '')
);

CREATE INDEX IF NOT EXISTS idx_operator_audit_events_created_at
ON authara.operator_audit_events (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_operator_audit_events_actor_user_id
ON authara.operator_audit_events (actor_user_id);

CREATE INDEX IF NOT EXISTS idx_operator_audit_events_resource
ON authara.operator_audit_events (resource_type, resource_id, created_at DESC);

INSERT INTO public.authara_schema_version (version)
VALUES (22)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DELETE FROM public.authara_schema_version
WHERE version = 22;

DROP INDEX IF EXISTS authara.idx_operator_audit_events_resource;
DROP INDEX IF EXISTS authara.idx_operator_audit_events_actor_user_id;
DROP INDEX IF EXISTS authara.idx_operator_audit_events_created_at;
DROP TABLE IF EXISTS authara.operator_audit_events;
