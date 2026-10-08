-- +migrate Up notransaction

-- Concurrent builds keep writes available while large production tables are
-- indexed. Dropping each known name first makes an interrupted non-transactional
-- migration safe to retry even when PostgreSQL left an invalid index behind.
DROP INDEX CONCURRENTLY IF EXISTS authara.idx_sessions_revoked_cleanup;
CREATE INDEX CONCURRENTLY idx_sessions_revoked_cleanup
ON authara.sessions (revoked_at, id)
WHERE revoked_at IS NOT NULL;

DROP INDEX CONCURRENTLY IF EXISTS authara.idx_refresh_tokens_expired_cleanup;
CREATE INDEX CONCURRENTLY idx_refresh_tokens_expired_cleanup
ON authara.refresh_tokens (expires_at, id)
WHERE consumed_at IS NULL;

DROP INDEX CONCURRENTLY IF EXISTS authara.idx_webauthn_challenges_consumed_cleanup;
CREATE INDEX CONCURRENTLY idx_webauthn_challenges_consumed_cleanup
ON authara.webauthn_challenges (consumed_at, id)
WHERE consumed_at IS NOT NULL;

DROP INDEX CONCURRENTLY IF EXISTS authara.idx_email_jobs_sent_cleanup;
CREATE INDEX CONCURRENTLY idx_email_jobs_sent_cleanup
ON authara.email_jobs (sent_at, id)
WHERE status = 'sent';

DROP INDEX CONCURRENTLY IF EXISTS authara.idx_email_jobs_failed_cleanup;
CREATE INDEX CONCURRENTLY idx_email_jobs_failed_cleanup
ON authara.email_jobs (COALESCE(failed_at, created_at), id)
WHERE status = 'failed';

INSERT INTO public.authara_schema_version (version)
VALUES (35)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DROP INDEX IF EXISTS authara.idx_email_jobs_failed_cleanup;
DROP INDEX IF EXISTS authara.idx_email_jobs_sent_cleanup;
DROP INDEX IF EXISTS authara.idx_webauthn_challenges_consumed_cleanup;
DROP INDEX IF EXISTS authara.idx_refresh_tokens_expired_cleanup;
DROP INDEX IF EXISTS authara.idx_sessions_revoked_cleanup;

DELETE FROM public.authara_schema_version
WHERE version = 35;
