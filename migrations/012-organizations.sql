-- +migrate Up

CREATE TABLE IF NOT EXISTS authara.organizations (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	name varchar(255) NOT NULL,
	kind varchar(50) NOT NULL,
	created_by_user_id uuid REFERENCES authara.users(id) ON DELETE SET NULL,

	CONSTRAINT organization_name_nonempty_check CHECK (length(btrim(name)) > 0),
	CONSTRAINT organization_kind_check CHECK (kind IN ('personal', 'team'))
);

DROP TRIGGER IF EXISTS trg_organization_updated_at ON authara.organizations;
CREATE TRIGGER trg_organization_updated_at
BEFORE UPDATE ON authara.organizations
FOR EACH ROW
EXECUTE FUNCTION authara.set_updated_at();

CREATE UNIQUE INDEX IF NOT EXISTS unique_personal_org_created_by_user
ON authara.organizations (created_by_user_id)
WHERE kind = 'personal' AND created_by_user_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_organizations_created_by_user_id
ON authara.organizations (created_by_user_id);

CREATE TABLE IF NOT EXISTS authara.organization_memberships (
	organization_id uuid NOT NULL REFERENCES authara.organizations(id) ON DELETE CASCADE,
	user_id uuid NOT NULL REFERENCES authara.users(id) ON DELETE CASCADE,
	role varchar(50) NOT NULL,
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),

	PRIMARY KEY (organization_id, user_id),
	CONSTRAINT organization_membership_role_check CHECK (role IN ('owner', 'admin', 'member'))
);

DROP TRIGGER IF EXISTS trg_organization_membership_updated_at ON authara.organization_memberships;
CREATE TRIGGER trg_organization_membership_updated_at
BEFORE UPDATE ON authara.organization_memberships
FOR EACH ROW
EXECUTE FUNCTION authara.set_updated_at();

CREATE INDEX IF NOT EXISTS idx_organization_memberships_user_id
ON authara.organization_memberships (user_id);

ALTER TABLE authara.sessions
ADD COLUMN active_organization_id uuid NOT NULL;

ALTER TABLE authara.sessions
ADD CONSTRAINT fk_sessions_active_organization
FOREIGN KEY (active_organization_id) REFERENCES authara.organizations(id);

ALTER TABLE authara.sessions
ADD CONSTRAINT fk_sessions_active_organization_membership
FOREIGN KEY (active_organization_id, user_id)
REFERENCES authara.organization_memberships(organization_id, user_id);

CREATE INDEX IF NOT EXISTS idx_sessions_active_organization_id
ON authara.sessions (active_organization_id);

ALTER TABLE authara.refresh_tokens
ADD COLUMN organization_id uuid NOT NULL;

ALTER TABLE authara.refresh_tokens
ADD CONSTRAINT fk_refresh_tokens_organization
FOREIGN KEY (organization_id) REFERENCES authara.organizations(id);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_organization_id
ON authara.refresh_tokens (organization_id);

INSERT INTO public.authara_schema_version (version)
VALUES (12)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DELETE FROM public.authara_schema_version
WHERE version = 12;

DROP INDEX IF EXISTS authara.idx_refresh_tokens_organization_id;
ALTER TABLE authara.refresh_tokens DROP CONSTRAINT IF EXISTS fk_refresh_tokens_organization;
ALTER TABLE authara.refresh_tokens DROP COLUMN IF EXISTS organization_id;

DROP INDEX IF EXISTS authara.idx_sessions_active_organization_id;
ALTER TABLE authara.sessions DROP CONSTRAINT IF EXISTS fk_sessions_active_organization_membership;
ALTER TABLE authara.sessions DROP CONSTRAINT IF EXISTS fk_sessions_active_organization;
ALTER TABLE authara.sessions DROP COLUMN IF EXISTS active_organization_id;

DROP INDEX IF EXISTS authara.idx_organization_memberships_user_id;
DROP TABLE IF EXISTS authara.organization_memberships;

DROP INDEX IF EXISTS authara.idx_organizations_created_by_user_id;
DROP INDEX IF EXISTS authara.unique_personal_org_created_by_user;
DROP TABLE IF EXISTS authara.organizations;
