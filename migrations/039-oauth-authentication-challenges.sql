-- +migrate Up

CREATE TABLE authara.oauth_authentication_challenges (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at timestamptz NOT NULL DEFAULT now(),
    provider text NOT NULL CHECK (provider IN ('google', 'apple', 'microsoft')),
    state text NOT NULL,
    nonce text NOT NULL,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz
);

CREATE INDEX oauth_authentication_challenges_cleanup_idx
    ON authara.oauth_authentication_challenges (expires_at, id);

INSERT INTO public.authara_schema_version (version)
VALUES (39)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DELETE FROM public.authara_schema_version
WHERE version = 39;

DROP TABLE authara.oauth_authentication_challenges;
