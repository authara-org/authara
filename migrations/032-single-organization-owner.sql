-- +migrate Up

CREATE UNIQUE INDEX organization_memberships_single_owner
ON authara.organization_memberships (organization_id)
WHERE role = 'owner';

INSERT INTO public.authara_schema_version (version)
VALUES (32)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DELETE FROM public.authara_schema_version
WHERE version = 32;

DROP INDEX IF EXISTS authara.organization_memberships_single_owner;
