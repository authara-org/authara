-- +migrate Up

ALTER TABLE authara.organization_invitations
ADD COLUMN IF NOT EXISTS metadata jsonb NOT NULL DEFAULT '{}'::jsonb;

INSERT INTO public.authara_schema_version (version)
VALUES (18)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DELETE FROM public.authara_schema_version
WHERE version = 18;

ALTER TABLE authara.organization_invitations
DROP COLUMN IF EXISTS metadata;
