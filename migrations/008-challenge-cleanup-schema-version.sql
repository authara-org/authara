-- +migrate Up

CREATE INDEX IF NOT EXISTS idx_challenges_expires_at
ON authara.challenges (expires_at);

CREATE INDEX IF NOT EXISTS idx_challenges_consumed_at
ON authara.challenges (consumed_at)
WHERE consumed_at IS NOT NULL;

INSERT INTO public.authara_schema_version (version)
VALUES (8)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DELETE FROM public.authara_schema_version
WHERE version = 8;

DROP INDEX IF EXISTS authara.idx_challenges_consumed_at;
DROP INDEX IF EXISTS authara.idx_challenges_expires_at;
