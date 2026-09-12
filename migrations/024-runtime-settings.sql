-- +migrate Up

CREATE TABLE IF NOT EXISTS authara.runtime_settings_state (
	singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
	revision bigint NOT NULL DEFAULT 0 CHECK (revision >= 0)
);

INSERT INTO authara.runtime_settings_state (singleton, revision)
VALUES (true, 0)
ON CONFLICT (singleton) DO NOTHING;

CREATE TABLE IF NOT EXISTS authara.runtime_setting_overrides (
	setting_key varchar(160) PRIMARY KEY,
	value jsonb NOT NULL,
	revision bigint NOT NULL CHECK (revision > 0),
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),
	updated_by_user_id uuid REFERENCES authara.users(id) ON DELETE SET NULL,

	CONSTRAINT runtime_setting_key_not_blank CHECK (btrim(setting_key) <> '')
);

DROP TRIGGER IF EXISTS trg_runtime_setting_override_updated_at ON authara.runtime_setting_overrides;
CREATE TRIGGER trg_runtime_setting_override_updated_at
BEFORE UPDATE ON authara.runtime_setting_overrides
FOR EACH ROW
EXECUTE FUNCTION authara.set_updated_at();

ALTER TABLE authara.challenges
ADD COLUMN IF NOT EXISTS minimum_resend_interval_ns bigint
CHECK (minimum_resend_interval_ns >= 0);

INSERT INTO public.authara_schema_version (version)
VALUES (24)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DELETE FROM public.authara_schema_version
WHERE version = 24;

ALTER TABLE authara.challenges
DROP COLUMN IF EXISTS minimum_resend_interval_ns;

DROP TRIGGER IF EXISTS trg_runtime_setting_override_updated_at ON authara.runtime_setting_overrides;
DROP TABLE IF EXISTS authara.runtime_setting_overrides;
DROP TABLE IF EXISTS authara.runtime_settings_state;
