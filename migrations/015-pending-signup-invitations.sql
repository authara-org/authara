-- +migrate Up

ALTER TABLE authara.pending_signup_actions
ADD COLUMN IF NOT EXISTS invitation_id uuid REFERENCES authara.organization_invitations(id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS idx_pending_signup_actions_invitation_id
ON authara.pending_signup_actions (invitation_id);

INSERT INTO public.authara_schema_version (version)
VALUES (15)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DELETE FROM public.authara_schema_version
WHERE version = 15;

DROP INDEX IF EXISTS authara.idx_pending_signup_actions_invitation_id;
ALTER TABLE authara.pending_signup_actions DROP COLUMN IF EXISTS invitation_id;
