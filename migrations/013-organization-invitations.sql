-- +migrate Up

CREATE TABLE IF NOT EXISTS authara.organization_invitations (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	organization_id uuid NOT NULL REFERENCES authara.organizations(id) ON DELETE CASCADE,
	email varchar(255) NOT NULL,
	role varchar(50) NOT NULL,
	token_hash varchar(512) NOT NULL UNIQUE,
	invited_by_user_id uuid REFERENCES authara.users(id) ON DELETE SET NULL,
	expires_at timestamptz NOT NULL,
	accepted_at timestamptz,
	accepted_by_user_id uuid REFERENCES authara.users(id) ON DELETE SET NULL,
	revoked_at timestamptz,
	revoked_by_user_id uuid REFERENCES authara.users(id) ON DELETE SET NULL,

	CONSTRAINT organization_invitation_email_nonempty_check CHECK (length(btrim(email)) > 0),
	CONSTRAINT organization_invitation_role_check CHECK (role IN ('owner', 'admin', 'member')),
	CONSTRAINT organization_invitation_not_accepted_and_revoked CHECK (accepted_at IS NULL OR revoked_at IS NULL),
	CONSTRAINT organization_invitation_accepted_by_required CHECK (accepted_at IS NULL OR accepted_by_user_id IS NOT NULL)
);

DROP TRIGGER IF EXISTS trg_organization_invitation_updated_at ON authara.organization_invitations;
CREATE TRIGGER trg_organization_invitation_updated_at
BEFORE UPDATE ON authara.organization_invitations
FOR EACH ROW
EXECUTE FUNCTION authara.set_updated_at();

CREATE INDEX IF NOT EXISTS idx_organization_invitations_organization_id
ON authara.organization_invitations (organization_id);

CREATE INDEX IF NOT EXISTS idx_organization_invitations_email_lower
ON authara.organization_invitations (lower(email));

CREATE INDEX IF NOT EXISTS idx_organization_invitations_expires_at
ON authara.organization_invitations (expires_at);

CREATE INDEX IF NOT EXISTS idx_organization_invitations_accepted_at
ON authara.organization_invitations (accepted_at);

CREATE INDEX IF NOT EXISTS idx_organization_invitations_revoked_at
ON authara.organization_invitations (revoked_at);

CREATE UNIQUE INDEX IF NOT EXISTS unique_active_organization_invitation_email
ON authara.organization_invitations (organization_id, lower(email))
WHERE accepted_at IS NULL AND revoked_at IS NULL;

INSERT INTO public.authara_schema_version (version)
VALUES (13)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DELETE FROM public.authara_schema_version
WHERE version = 13;

DROP INDEX IF EXISTS authara.unique_active_organization_invitation_email;
DROP INDEX IF EXISTS authara.idx_organization_invitations_revoked_at;
DROP INDEX IF EXISTS authara.idx_organization_invitations_accepted_at;
DROP INDEX IF EXISTS authara.idx_organization_invitations_expires_at;
DROP INDEX IF EXISTS authara.idx_organization_invitations_email_lower;
DROP INDEX IF EXISTS authara.idx_organization_invitations_organization_id;
DROP TABLE IF EXISTS authara.organization_invitations;
