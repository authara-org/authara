-- +migrate Up

INSERT INTO authara.platform_roles (name)
VALUES ('operator')
ON CONFLICT (name) DO NOTHING;

INSERT INTO public.authara_schema_version (version)
VALUES (19)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DELETE FROM public.authara_schema_version
WHERE version = 19;

DELETE FROM authara.platform_roles
WHERE name = 'operator';
