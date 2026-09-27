-- +migrate Up

ALTER TABLE authara.security_events
ALTER COLUMN response DROP NOT NULL;

ALTER TABLE authara.security_events
ADD COLUMN outcome varchar(32) NOT NULL DEFAULT 'success',
ADD COLUMN reason_code varchar(64),
ADD COLUMN actor_type varchar(32) NOT NULL DEFAULT 'system',
ADD COLUMN actor_user_id uuid,
ADD COLUMN session_id uuid,
ADD COLUMN organization_id uuid,
ADD COLUMN authentication_method varchar(32);

ALTER TABLE authara.security_events
DROP CONSTRAINT IF EXISTS security_events_user_id_fkey,
DROP CONSTRAINT IF EXISTS security_events_passkey_id_fkey;

ALTER TABLE authara.security_events
ADD CONSTRAINT security_events_outcome_check
CHECK (outcome IN ('success', 'denied', 'failure')),
ADD CONSTRAINT security_events_actor_type_check
CHECK (actor_type IN ('anonymous', 'user', 'system')),
ADD CONSTRAINT security_events_type_check
CHECK (type IN (
	'authentication.login',
	'authentication.reauthenticated',
	'session.refresh',
	'session.refresh_token_reuse',
	'session.logout',
	'session.revoked',
	'credential.password_added',
	'credential.password_changed',
	'credential.password_reset',
	'credential.provider_linked',
	'credential.provider_changed',
	'credential.provider_removed',
	'credential.passkey_added',
	'credential.passkey_removed',
	'account.email_changed',
	'passkey.clone_warning'
)),
ADD CONSTRAINT security_events_reason_code_check
CHECK (reason_code IS NULL OR reason_code IN ('invalid_credentials', 'invalid_assertion', 'access_policy', 'account_link_required', 'provider_disabled', 'audience_forbidden', 'user_disabled', 'authentication_method_unavailable', 'refresh_token_reuse', 'user_requested', 'security_containment')),
ADD CONSTRAINT security_events_authentication_method_check
CHECK (authentication_method IS NULL OR authentication_method IN ('password', 'passkey', 'google')),
ADD CONSTRAINT security_events_response_check
CHECK (response IS NULL OR response IN ('password', 'google', 'alert', 'restrict', 'restrict_and_revoke'));

CREATE INDEX idx_security_events_type_created_at
ON authara.security_events (type, created_at DESC, id DESC);

CREATE INDEX idx_security_events_session_id_created_at
ON authara.security_events (session_id, created_at DESC, id DESC)
WHERE session_id IS NOT NULL;

CREATE INDEX idx_security_events_outcome_created_at
ON authara.security_events (outcome, created_at DESC, id DESC);

INSERT INTO public.authara_schema_version (version)
VALUES (30)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DELETE FROM public.authara_schema_version
WHERE version = 30;

DROP INDEX IF EXISTS authara.idx_security_events_outcome_created_at;
DROP INDEX IF EXISTS authara.idx_security_events_session_id_created_at;
DROP INDEX IF EXISTS authara.idx_security_events_type_created_at;

ALTER TABLE authara.security_events
DROP CONSTRAINT IF EXISTS security_events_response_check,
DROP CONSTRAINT IF EXISTS security_events_authentication_method_check,
DROP CONSTRAINT IF EXISTS security_events_reason_code_check,
DROP CONSTRAINT IF EXISTS security_events_type_check,
DROP CONSTRAINT IF EXISTS security_events_actor_type_check,
DROP CONSTRAINT IF EXISTS security_events_outcome_check;

ALTER TABLE authara.security_events
DROP COLUMN IF EXISTS authentication_method,
DROP COLUMN IF EXISTS organization_id,
DROP COLUMN IF EXISTS session_id,
DROP COLUMN IF EXISTS actor_user_id,
DROP COLUMN IF EXISTS actor_type,
DROP COLUMN IF EXISTS reason_code,
DROP COLUMN IF EXISTS outcome;

UPDATE authara.security_events
SET response = ''
WHERE response IS NULL;

ALTER TABLE authara.security_events
ALTER COLUMN response SET NOT NULL;

UPDATE authara.security_events AS event
SET user_id = NULL
WHERE user_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM authara.users WHERE users.id = event.user_id);

UPDATE authara.security_events AS event
SET passkey_id = NULL
WHERE passkey_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM authara.passkeys WHERE passkeys.id = event.passkey_id);

ALTER TABLE authara.security_events
ADD CONSTRAINT security_events_user_id_fkey
FOREIGN KEY (user_id) REFERENCES authara.users(id) ON DELETE SET NULL,
ADD CONSTRAINT security_events_passkey_id_fkey
FOREIGN KEY (passkey_id) REFERENCES authara.passkeys(id) ON DELETE SET NULL;
