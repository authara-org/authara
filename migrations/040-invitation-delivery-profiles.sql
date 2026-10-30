-- +migrate Up

ALTER TABLE authara.organization_invitations
ADD COLUMN delivery_profile varchar(64) NOT NULL DEFAULT 'authara-hosted';

ALTER TABLE authara.organization_invitations
ADD CONSTRAINT organization_invitation_delivery_profile_check
CHECK (delivery_profile ~ '^[a-z0-9][a-z0-9_-]{0,63}$');

INSERT INTO public.authara_schema_version (version)
VALUES (40)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DELETE FROM public.authara_schema_version
WHERE version = 40;

ALTER TABLE authara.organization_invitations
DROP CONSTRAINT IF EXISTS organization_invitation_delivery_profile_check;

ALTER TABLE authara.organization_invitations
DROP COLUMN IF EXISTS delivery_profile;
