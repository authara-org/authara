-- +migrate Up

-- USERS
-- Already UNIQUE(email) exists, but we explicitly ensure index exists
CREATE INDEX IF NOT EXISTS idx_users_email
ON authara.users (email);

-- AUTH PROVIDERS
CREATE INDEX IF NOT EXISTS idx_auth_providers_user_provider
ON authara.auth_providers (user_id, provider);

-- USER ROLES
CREATE INDEX IF NOT EXISTS idx_user_platform_roles_user_id
ON authara.user_platform_roles (user_id);

-- REFRESH TOKENS (critical path)
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_token_hash
ON authara.refresh_tokens (token_hash);

-- SESSIONS CLEANUP
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at
ON authara.sessions (expires_at);

-- USER SESSIONS
CREATE INDEX IF NOT EXISTS idx_sessions_user_id
ON authara.sessions (user_id);

-- REFRESH TOKENS CLEANUP
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires_at
ON authara.refresh_tokens (expires_at);

-- REFRESH TOKEN WITH SESSION ID
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_session_id
ON authara.refresh_tokens(session_id);

-- +migrate Down

DROP INDEX IF EXISTS authara.idx_refresh_tokens_expires_at;
DROP INDEX IF EXISTS authara.idx_sessions_expires_at;
DROP INDEX IF EXISTS authara.idx_refresh_tokens_token_hash;
DROP INDEX IF EXISTS authara.idx_user_platform_roles_user_id;
DROP INDEX IF EXISTS authara.idx_auth_providers_user_provider;
DROP INDEX IF EXISTS authara.idx_users_email;
DROP INDEX IF EXISTS authara.idx_refresh_tokens_session_id;
