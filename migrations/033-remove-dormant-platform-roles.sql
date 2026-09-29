-- +migrate Up

-- These roles never had route permissions or a supported assignment surface.
-- Deleting them also deletes any manual assignments through the foreign key.
DELETE FROM authara.platform_roles
WHERE name IN ('auditor', 'monitor');

INSERT INTO public.authara_schema_version (version)
VALUES (33)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DELETE FROM public.authara_schema_version
WHERE version = 33;

INSERT INTO authara.platform_roles (name)
VALUES
  ('auditor'),
  ('monitor')
ON CONFLICT (name) DO NOTHING;
