-- +migrate Up

CREATE TABLE IF NOT EXISTS authara.webhook_events (
	id varchar(64) PRIMARY KEY,
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),

	event_type varchar(128) NOT NULL,
	payload jsonb NOT NULL,
	status varchar(32) NOT NULL DEFAULT 'pending',

	attempt_count integer NOT NULL DEFAULT 0,
	processing_started_at timestamptz,
	last_error text,
	delivered_at timestamptz,

	CONSTRAINT webhook_events_status_check
		CHECK (status IN ('pending', 'processing', 'delivered', 'failed'))
);

DROP TRIGGER IF EXISTS trg_webhook_event_updated_at ON authara.webhook_events;
CREATE TRIGGER trg_webhook_event_updated_at
BEFORE UPDATE ON authara.webhook_events
FOR EACH ROW
EXECUTE FUNCTION authara.set_updated_at();

CREATE INDEX IF NOT EXISTS idx_webhook_events_pending
ON authara.webhook_events (created_at, id)
WHERE status = 'pending';

INSERT INTO public.authara_schema_version (version)
VALUES (16)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DELETE FROM public.authara_schema_version
WHERE version = 16;

DROP INDEX IF EXISTS authara.idx_webhook_events_pending;
DROP TABLE IF EXISTS authara.webhook_events;
