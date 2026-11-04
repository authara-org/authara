-- +migrate Up

-- +migrate StatementBegin
CREATE OR REPLACE FUNCTION authara.notify_webhook_event_ready()
RETURNS trigger AS $func$
BEGIN
	PERFORM pg_notify('authara_webhook_events_ready', '');
	RETURN NEW;
END;
$func$ LANGUAGE plpgsql;
-- +migrate StatementEnd

CREATE TRIGGER trg_webhook_event_ready
AFTER INSERT ON authara.webhook_events
FOR EACH ROW
EXECUTE FUNCTION authara.notify_webhook_event_ready();

INSERT INTO public.authara_schema_version (version)
VALUES (41)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DELETE FROM public.authara_schema_version
WHERE version = 41;

DROP TRIGGER IF EXISTS trg_webhook_event_ready ON authara.webhook_events;
DROP FUNCTION IF EXISTS authara.notify_webhook_event_ready();
