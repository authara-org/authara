-- +migrate Up

ALTER TABLE authara.sessions
DROP CONSTRAINT chk_sessions_authentication_provenance,
ADD CONSTRAINT chk_sessions_authentication_provenance CHECK (
	(authenticated_at IS NULL AND authentication_method IS NULL) OR
	(authenticated_at IS NOT NULL AND authentication_method IN ('password', 'passkey', 'google', 'apple', 'microsoft'))
);

ALTER TABLE authara.authentication_challenges
DROP CONSTRAINT chk_authentication_challenge_completion,
ADD CONSTRAINT chk_authentication_challenge_completion CHECK (
	(consumed_at IS NULL AND authentication_method IS NULL) OR
	(consumed_at IS NOT NULL AND authentication_method IN ('password', 'passkey', 'google', 'apple', 'microsoft'))
);

ALTER TABLE authara.security_events
DROP CONSTRAINT security_events_authentication_method_check,
ADD CONSTRAINT security_events_authentication_method_check
	CHECK (authentication_method IS NULL OR authentication_method IN ('password', 'passkey', 'google', 'apple', 'microsoft')),
DROP CONSTRAINT security_events_response_check,
ADD CONSTRAINT security_events_response_check
	CHECK (response IS NULL OR response IN ('password', 'google', 'apple', 'microsoft', 'alert', 'restrict', 'restrict_and_revoke'));

ALTER TABLE authara.email_verification_transactions
DROP CONSTRAINT email_verification_transaction_authentication_method_check,
ADD CONSTRAINT email_verification_transaction_authentication_method_check
	CHECK (authentication_method IN ('password', 'passkey', 'google', 'apple', 'microsoft'));

INSERT INTO public.authara_schema_version (version)
VALUES (38)
ON CONFLICT (version) DO NOTHING;

-- +migrate Down

DELETE FROM public.authara_schema_version
WHERE version = 38;

ALTER TABLE authara.email_verification_transactions
DROP CONSTRAINT email_verification_transaction_authentication_method_check,
ADD CONSTRAINT email_verification_transaction_authentication_method_check
	CHECK (authentication_method IN ('password', 'passkey', 'google', 'apple'));

ALTER TABLE authara.security_events
DROP CONSTRAINT security_events_response_check,
ADD CONSTRAINT security_events_response_check
	CHECK (response IS NULL OR response IN ('password', 'google', 'apple', 'alert', 'restrict', 'restrict_and_revoke')),
DROP CONSTRAINT security_events_authentication_method_check,
ADD CONSTRAINT security_events_authentication_method_check
	CHECK (authentication_method IS NULL OR authentication_method IN ('password', 'passkey', 'google', 'apple'));

ALTER TABLE authara.authentication_challenges
DROP CONSTRAINT chk_authentication_challenge_completion,
ADD CONSTRAINT chk_authentication_challenge_completion CHECK (
	(consumed_at IS NULL AND authentication_method IS NULL) OR
	(consumed_at IS NOT NULL AND authentication_method IN ('password', 'passkey', 'google', 'apple'))
);

ALTER TABLE authara.sessions
DROP CONSTRAINT chk_sessions_authentication_provenance,
ADD CONSTRAINT chk_sessions_authentication_provenance CHECK (
	(authenticated_at IS NULL AND authentication_method IS NULL) OR
	(authenticated_at IS NOT NULL AND authentication_method IN ('password', 'passkey', 'google', 'apple'))
);
