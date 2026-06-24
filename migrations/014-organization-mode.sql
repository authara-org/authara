-- +migrate Up

CREATE TABLE IF NOT EXISTS authara.organization_mode (
	id integer PRIMARY KEY DEFAULT 1,
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	mode varchar(50) NOT NULL,

	CONSTRAINT organization_mode_singleton_check CHECK (id = 1),
	CONSTRAINT organization_mode_mode_check CHECK (mode IN ('personal', 'single', 'multi'))
);

DROP TRIGGER IF EXISTS trg_organization_mode_updated_at ON authara.organization_mode;
CREATE TRIGGER trg_organization_mode_updated_at
BEFORE UPDATE ON authara.organization_mode
FOR EACH ROW
EXECUTE FUNCTION authara.set_updated_at();

INSERT INTO public.authara_schema_version (version)
VALUES (14)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DELETE FROM public.authara_schema_version
WHERE version = 14;

DROP TRIGGER IF EXISTS trg_organization_mode_updated_at ON authara.organization_mode;
DROP TABLE IF EXISTS authara.organization_mode;
