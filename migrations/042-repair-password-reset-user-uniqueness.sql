-- +migrate Up

WITH ranked_resets AS (
	SELECT id,
		row_number() OVER (
			PARTITION BY user_id
			ORDER BY created_at DESC, id DESC
		) AS position
	FROM authara.pending_password_resets
)
DELETE FROM authara.pending_password_resets AS reset
USING ranked_resets
WHERE reset.id = ranked_resets.id
	AND ranked_resets.position > 1;

-- +migrate StatementBegin
DO $migration$
BEGIN
	IF NOT EXISTS (
		SELECT 1
		FROM pg_constraint
		WHERE conrelid = 'authara.pending_password_resets'::regclass
			AND conname = 'unique_pending_password_reset_user'
	) THEN
		ALTER TABLE authara.pending_password_resets
			ADD CONSTRAINT unique_pending_password_reset_user UNIQUE (user_id);
	END IF;
END;
$migration$;
-- +migrate StatementEnd

INSERT INTO public.authara_schema_version (version)
VALUES (42)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DELETE FROM public.authara_schema_version
WHERE version = 42;

-- Older databases retain this index from the original migration 005.
-- +migrate StatementBegin
DO $migration$
BEGIN
	IF to_regclass('authara.idx_pending_password_resets_user_id') IS NOT NULL THEN
		ALTER TABLE authara.pending_password_resets
			DROP CONSTRAINT IF EXISTS unique_pending_password_reset_user;
	END IF;
END;
$migration$;
-- +migrate StatementEnd
