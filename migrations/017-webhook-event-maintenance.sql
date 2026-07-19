-- +migrate Up

ALTER TABLE authara.webhook_events
ADD COLUMN IF NOT EXISTS next_attempt_at timestamptz NOT NULL DEFAULT now();

DROP INDEX IF EXISTS authara.idx_webhook_events_pending;

CREATE INDEX IF NOT EXISTS idx_webhook_events_pending
ON authara.webhook_events (next_attempt_at, created_at, id)
WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_webhook_events_processing
ON authara.webhook_events (processing_started_at, id)
WHERE status = 'processing';

CREATE INDEX IF NOT EXISTS idx_webhook_events_terminal
ON authara.webhook_events (updated_at, id)
WHERE status IN ('delivered', 'failed');

INSERT INTO public.authara_schema_version (version)
VALUES (17)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DELETE FROM public.authara_schema_version
WHERE version = 17;

DROP INDEX IF EXISTS authara.idx_webhook_events_terminal;
DROP INDEX IF EXISTS authara.idx_webhook_events_processing;

DROP INDEX IF EXISTS authara.idx_webhook_events_pending;
CREATE INDEX IF NOT EXISTS idx_webhook_events_pending
ON authara.webhook_events (created_at, id)
WHERE status = 'pending';

ALTER TABLE authara.webhook_events
DROP COLUMN IF EXISTS next_attempt_at;
