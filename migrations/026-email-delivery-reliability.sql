-- +migrate Up

ALTER TABLE authara.email_jobs
ADD COLUMN delivery_deadline_at timestamptz,
ADD COLUMN terminal_reason varchar(64),
ADD COLUMN failed_at timestamptz;

UPDATE authara.email_jobs AS job
SET delivery_deadline_at = job.created_at + interval '72 hours';

UPDATE authara.email_jobs AS job
SET delivery_deadline_at = challenge.expires_at
FROM authara.challenges AS challenge
WHERE challenge.id = job.challenge_id;

UPDATE authara.email_jobs
SET terminal_reason = 'legacy_failure',
	failed_at = updated_at
WHERE status = 'failed';

ALTER TABLE authara.email_jobs
ALTER COLUMN delivery_deadline_at SET DEFAULT (now() + interval '72 hours'),
ALTER COLUMN delivery_deadline_at SET NOT NULL;

CREATE INDEX idx_email_jobs_processing
ON authara.email_jobs (processing_started_at, id)
WHERE status = 'processing';

INSERT INTO public.authara_schema_version (version)
VALUES (26)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DELETE FROM public.authara_schema_version
WHERE version = 26;

DROP INDEX IF EXISTS authara.idx_email_jobs_processing;

ALTER TABLE authara.email_jobs
DROP COLUMN IF EXISTS failed_at,
DROP COLUMN IF EXISTS terminal_reason,
DROP COLUMN IF EXISTS delivery_deadline_at;
