-- +migrate Up

ALTER TABLE authara.pending_provider_links
	ALTER COLUMN session_id DROP NOT NULL,
	ADD COLUMN IF NOT EXISTS challenge_id uuid REFERENCES authara.challenges(id) ON DELETE SET NULL,
	ADD COLUMN IF NOT EXISTS provider_user_id varchar(255),
	ADD COLUMN IF NOT EXISTS provider_email varchar(255),
	ADD COLUMN IF NOT EXISTS provider_email_verified boolean NOT NULL DEFAULT false,
	ADD COLUMN IF NOT EXISTS purpose varchar(64) NOT NULL DEFAULT 'authenticated_link';

ALTER TABLE authara.pending_provider_links
	DROP CONSTRAINT IF EXISTS chk_pending_provider_links_purpose;

ALTER TABLE authara.pending_provider_links
	ADD CONSTRAINT chk_pending_provider_links_purpose
	CHECK (purpose IN ('authenticated_link', 'account_recovery_link'));

CREATE INDEX IF NOT EXISTS idx_pending_provider_links_provider_user_id
ON authara.pending_provider_links (provider, provider_user_id);

CREATE INDEX IF NOT EXISTS idx_pending_provider_links_expires_at
ON authara.pending_provider_links (expires_at);

INSERT INTO public.authara_schema_version (version)
VALUES (9)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DELETE FROM public.authara_schema_version
WHERE version = 9;

DROP INDEX IF EXISTS authara.idx_pending_provider_links_expires_at;
DROP INDEX IF EXISTS authara.idx_pending_provider_links_provider_user_id;

ALTER TABLE authara.pending_provider_links
	DROP CONSTRAINT IF EXISTS chk_pending_provider_links_purpose;

ALTER TABLE authara.pending_provider_links
	DROP COLUMN IF EXISTS purpose,
	DROP COLUMN IF EXISTS provider_email_verified,
	DROP COLUMN IF EXISTS provider_email,
	DROP COLUMN IF EXISTS provider_user_id,
	DROP COLUMN IF EXISTS challenge_id,
	ALTER COLUMN session_id SET NOT NULL;
