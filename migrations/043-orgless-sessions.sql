-- +migrate Up

ALTER TABLE authara.sessions
	ALTER COLUMN active_organization_id DROP NOT NULL;

ALTER TABLE authara.refresh_tokens
	ALTER COLUMN organization_id DROP NOT NULL;

INSERT INTO public.authara_schema_version (version)
VALUES (43)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DELETE FROM authara.refresh_tokens
WHERE organization_id IS NULL;

DELETE FROM authara.sessions
WHERE active_organization_id IS NULL;

ALTER TABLE authara.refresh_tokens
	ALTER COLUMN organization_id SET NOT NULL;

ALTER TABLE authara.sessions
	ALTER COLUMN active_organization_id SET NOT NULL;

DELETE FROM public.authara_schema_version
WHERE version = 43;
