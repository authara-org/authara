# Core security events

Authara persists authentication, session, and credential lifecycle events in
the append-only `security_events` store. Event writes participate in the same
database transaction as successful state changes. Denied authentication
events are committed separately from the error returned to the client so that
the denial remains durable.

Each event records an event type, outcome, actor kind, optional actor and
subject user IDs, optional session/organization/passkey IDs, an authentication
method, and an allowlisted reason or response code. Events never contain email
addresses, submitted identifiers, passwords, tokens, challenge codes,
credential bytes, IP addresses, or user-agent strings.

The `internal/securityevent` module owns typed recording methods, filtered
queries, NDJSON export, and cleanup. Domain services receive one shared recorder
and never construct generic event rows directly.

`AUTHARA_SECURITY_EVENT_ENABLED_EVENTS` is a comma-separated allowlist. When it
is unset, Authara uses the standard set, which excludes routine successful
refresh and logout events. `none` disables persistence and `all` enables every
supported event. Unknown or duplicate names fail startup validation.

`AUTHARA_SECURITY_EVENT_RETENTION_DAYS` controls independent security-event
retention and defaults to 180 days. A dedicated daily cleanup worker applies
the configured cutoff.
