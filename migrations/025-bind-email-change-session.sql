-- +migrate Up

-- Existing email-change challenges cannot be safely attributed to a session.
DELETE FROM authara.challenges c
USING authara.pending_email_changes p
WHERE c.id = p.challenge_id;

ALTER TABLE authara.pending_email_changes
ADD COLUMN initiating_session_id uuid NOT NULL
REFERENCES authara.sessions(id) ON DELETE CASCADE;

CREATE INDEX idx_pending_email_changes_initiating_session_id
ON authara.pending_email_changes (initiating_session_id);

-- +migrate StatementBegin
CREATE FUNCTION authara.cancel_email_changes_for_revoked_session()
RETURNS trigger AS $func$
BEGIN
  DELETE FROM authara.pending_email_changes
  WHERE initiating_session_id = NEW.id;
  RETURN NEW;
END;
$func$ LANGUAGE plpgsql;
-- +migrate StatementEnd

CREATE TRIGGER trg_cancel_email_changes_for_revoked_session
AFTER UPDATE OF revoked_at ON authara.sessions
FOR EACH ROW
WHEN (OLD.revoked_at IS NULL AND NEW.revoked_at IS NOT NULL)
EXECUTE FUNCTION authara.cancel_email_changes_for_revoked_session();

-- +migrate StatementBegin
CREATE FUNCTION authara.cancel_email_changes_for_disabled_user()
RETURNS trigger AS $func$
BEGIN
  DELETE FROM authara.pending_email_changes
  WHERE user_id = NEW.id;
  RETURN NEW;
END;
$func$ LANGUAGE plpgsql;
-- +migrate StatementEnd

CREATE TRIGGER trg_cancel_email_changes_for_disabled_user
AFTER UPDATE OF disabled_at ON authara.users
FOR EACH ROW
WHEN (OLD.disabled_at IS NULL AND NEW.disabled_at IS NOT NULL)
EXECUTE FUNCTION authara.cancel_email_changes_for_disabled_user();

INSERT INTO public.authara_schema_version (version)
VALUES (25)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

-- Do not expose session-bound actions to older code that cannot enforce the binding.
DELETE FROM authara.challenges c
USING authara.pending_email_changes p
WHERE c.id = p.challenge_id;

DELETE FROM public.authara_schema_version
WHERE version = 25;

DROP TRIGGER IF EXISTS trg_cancel_email_changes_for_disabled_user ON authara.users;
DROP FUNCTION IF EXISTS authara.cancel_email_changes_for_disabled_user();

DROP TRIGGER IF EXISTS trg_cancel_email_changes_for_revoked_session ON authara.sessions;
DROP FUNCTION IF EXISTS authara.cancel_email_changes_for_revoked_session();

DROP INDEX IF EXISTS authara.idx_pending_email_changes_initiating_session_id;

ALTER TABLE authara.pending_email_changes
DROP COLUMN IF EXISTS initiating_session_id;
