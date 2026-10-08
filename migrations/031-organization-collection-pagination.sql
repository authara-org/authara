-- +migrate Up

CREATE INDEX IF NOT EXISTS idx_organization_memberships_org_created_user
ON authara.organization_memberships (organization_id, created_at ASC, user_id ASC);

CREATE INDEX IF NOT EXISTS idx_organization_memberships_user_created_org
ON authara.organization_memberships (user_id, created_at ASC, organization_id ASC);

CREATE INDEX IF NOT EXISTS idx_organization_invitations_org_created_id
ON authara.organization_invitations (organization_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_sessions_user_created_id
ON authara.sessions (user_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_active_sessions_user_created_id
ON authara.sessions (user_id, created_at DESC, id DESC)
WHERE revoked_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_passkeys_user_created_id
ON authara.passkeys (user_id, created_at ASC, id ASC);

INSERT INTO public.authara_schema_version (version)
VALUES (31)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DELETE FROM public.authara_schema_version
WHERE version = 31;

DROP INDEX IF EXISTS authara.idx_organization_invitations_org_created_id;
DROP INDEX IF EXISTS authara.idx_passkeys_user_created_id;
DROP INDEX IF EXISTS authara.idx_active_sessions_user_created_id;
DROP INDEX IF EXISTS authara.idx_sessions_user_created_id;
DROP INDEX IF EXISTS authara.idx_organization_memberships_user_created_org;
DROP INDEX IF EXISTS authara.idx_organization_memberships_org_created_user;
