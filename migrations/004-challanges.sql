-- +migrate Up

CREATE TABLE IF NOT EXISTS authara.challenges (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),

	purpose varchar(64) NOT NULL,
	email varchar(255) NOT NULL,

	expires_at timestamptz NOT NULL,
	consumed_at timestamptz,

	attempt_count integer NOT NULL DEFAULT 0,
	max_attempts integer NOT NULL DEFAULT 5,

	resend_count integer NOT NULL DEFAULT 0,
	max_resends integer NOT NULL DEFAULT 3,
	last_sent_at timestamptz
);

DROP TRIGGER IF EXISTS trg_challenge_updated_at ON authara.challenges;
CREATE TRIGGER trg_challenge_updated_at
BEFORE UPDATE ON authara.challenges
FOR EACH ROW
EXECUTE FUNCTION authara.set_updated_at();


CREATE TABLE IF NOT EXISTS authara.verification_codes (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	created_at timestamptz NOT NULL DEFAULT now(),

	challenge_id uuid NOT NULL REFERENCES authara.challenges(id) ON DELETE CASCADE,
	code_hash varchar(255) NOT NULL,
	expires_at timestamptz NOT NULL,

	CONSTRAINT unique_verification_code_challenge UNIQUE (challenge_id)
);


CREATE TABLE IF NOT EXISTS authara.email_jobs (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now(),

	challenge_id uuid REFERENCES authara.challenges(id) ON DELETE CASCADE,
	to_email varchar(255) NOT NULL,
	template varchar(64) NOT NULL,
	template_data jsonb,

	status varchar(32) NOT NULL,

	attempt_count integer NOT NULL DEFAULT 0,
	next_attempt_at timestamptz NOT NULL DEFAULT now(),
	processing_started_at timestamptz,
	last_error text,
	sent_at timestamptz
);

DROP TRIGGER IF EXISTS trg_email_job_updated_at ON authara.email_jobs;
CREATE TRIGGER trg_email_job_updated_at
BEFORE UPDATE ON authara.email_jobs
FOR EACH ROW
EXECUTE FUNCTION authara.set_updated_at();


-- critical index for worker
CREATE INDEX IF NOT EXISTS idx_email_jobs_pending
ON authara.email_jobs (next_attempt_at, created_at)
WHERE status = 'pending';


CREATE TABLE IF NOT EXISTS authara.pending_signup_actions (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	created_at timestamptz NOT NULL DEFAULT now(),

	challenge_id uuid NOT NULL REFERENCES authara.challenges(id) ON DELETE CASCADE,
	email varchar(255) NOT NULL,
	username varchar(255) NOT NULL,
	password_hash varchar(255) NOT NULL,

	CONSTRAINT unique_pending_signup_challenge UNIQUE (challenge_id)
);


-- +migrate Down

DROP TABLE IF EXISTS authara.pending_signup_actions;
DROP INDEX IF EXISTS authara.idx_email_jobs_pending;
DROP TABLE IF EXISTS authara.email_jobs;
DROP TABLE IF EXISTS authara.verification_codes;
DROP TABLE IF EXISTS authara.challenges;
