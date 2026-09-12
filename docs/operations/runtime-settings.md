# Operator runtime settings

Authara has a configuration inventory and curated runtime-policy control plane at:

```text
/auth/operator/settings
```

Only users with the exact `authara:operator` platform role can access or
change these settings. Mutations use the same CSRF protection as other
operator forms, validate a registered typed value and the complete related
policy, use optimistic revisions, and create an operator audit event in the
same PostgreSQL statement as the override.

The page lists every environment variable consumed by Core, grouped by its
configuration area. Each entry reports whether the variable was explicitly
set, whether a built-in default is active, whether it is required, its
effective non-secret value, and whether it can be changed by an operator.
Unset optional variables without defaults are reported separately from values
using a default.

This is not a generic environment editor. Database, cache, SMTP, OAuth, JWT,
internal-token, provider, worker, server, metrics, proxy, and other bootstrap
settings remain deployment-controlled and require a restart to change.
Sensitive values are reduced to presence information before the settings
snapshot is built: the page can report that one is configured but never holds
or renders its value. Email-template content and per-template delivery remain
operator-only settings in their purpose-built tables.

Internally, Core exposes one `internal/config.Service`. It owns the immutable
startup configuration as well as the resolved live-policy snapshots. Other
services receive that config service, or a narrow typed policy-reader
interface implemented by it, and do not inspect whether an effective value
came from the environment, an operator override, or a built-in default. The
database persistence DTOs live in a dependency-free config subpackage only to
avoid an import cycle with the store; there is no second settings service.

## Resolution and environment locks

Hybrid settings resolve in this order:

1. an explicitly supplied environment variable;
2. a persisted operator override;
3. the built-in default.

Authara uses environment lookup, not the value returned by the environment
parser, to decide whether an environment variable is explicit. An absent
variable whose parser tag supplies a default therefore does not lock the
setting.

An explicit environment value locks its hybrid setting. The page shows the
effective value as **Environment**, rejects set and clear requests on the
server, and explains that deployment configuration and a restart are needed.
A previous operator override remains dormant in PostgreSQL. If the environment
value is removed on a later restart, that override becomes effective again.

**Reset to built-in default** deletes the override rather than storing a copy
of today's default. Future default changes can therefore take effect.

## Dynamic challenge policy

The first runtime policy group contains:

| Setting | Operator range | Effect |
| --- | --- | --- |
| `AUTHARA_CHALLENGE_TTL` | 5m–24h | New challenges |
| `AUTHARA_CHALLENGE_VERIFICATION_CODE_TTL` | 1m–1h and no longer than the challenge lifetime | Newly generated codes |
| `AUTHARA_CHALLENGE_MAX_ATTEMPTS` | 1–20 | New challenges |
| `AUTHARA_CHALLENGE_MAX_RESENDS` | 0–10 | New challenges |
| `AUTHARA_CHALLENGE_MIN_RESEND_INTERVAL` | 0s–15m | New challenges |

`AUTHARA_CHALLENGE_ENABLED` remains environment-only and startup-only because
it currently controls middleware, rendered authentication flows, and email
worker startup.

Each request reads one immutable challenge-policy snapshot at the beginning of
the operation. New challenges persist their expiry, attempt limit, resend
limit, and minimum resend interval. New verification codes persist their own
expiry. Later policy changes do not rewrite those artifacts or revoke anything
already issued.

Challenge rows created before schema version 24 have no stored resend interval.
Those legacy rows continue to use the current effective interval, matching the
behavior they had before this field was persisted. Challenges created by
version 24 and later retain the interval captured at creation.

## Dynamic rate-limit policy

All `AUTHARA_RATE_LIMIT_*` settings are hybrid and can be changed from the
operator page when their environment variable is absent. Counts accept
operator values from 1 through 1,000,000. Windows accept durations from one
second through seven days. The in-memory maximum-entry setting has a stricter
minimum of 100.

Each limiter call reads one immutable policy snapshot. A changed threshold
therefore applies to the next check, including a bucket that already exists.
A changed window applies when a bucket is next created; an existing in-memory
or Redis bucket keeps its current reset deadline. Lowering a limit does not
delete counters, and raising one does not reset them.

`AUTHARA_RATE_LIMIT_CLEANUP_EVERY` and `AUTHARA_RATE_LIMIT_MAX_ENTRIES` control
the in-memory limiter only. Redis-backed rate limiting ignores these two
housekeeping settings. As with challenge policy, an explicitly supplied
environment variable locks that individual setting until the variable is
removed and Core is restarted.

## Replicas and failure behavior

PostgreSQL is the source of truth. The replica accepting a successful mutation
publishes its new immutable in-memory snapshot before returning success. Other
replicas compare the global settings revision and reload every two seconds, so
they normally update on the next poll plus database/query scheduling. Failed
polls can extend that delay. There is no claim of simultaneous distributed
application.

Polling is also reconciliation: there is no notification that can be lost,
and a temporarily failed query is retried on the next interval. Reads on hot
paths only load an atomic in-memory snapshot and never query PostgreSQL.

Invalid values, unknown keys, locked settings, and stale revisions are rejected
without persistence or publication. Startup fails clearly if a stored override
is unknown, malformed, outside operator safety bounds, or makes a complete
typed policy invalid.

## Backup and rollback

Include these tables with normal PostgreSQL backups:

```text
authara.runtime_settings_state
authara.runtime_setting_overrides
authara.operator_audit_events
```

The normal rollback path is to clear an override in the operator page. A
deployment environment value can enforce an emergency value on every replica
after restart while preserving the dormant override for later review.

## Later policy groups

Token/session lifetimes and rotation, username login, organization invitation
policy, public organization management, webhook event selection,
access-policy enablement, and applicable retention periods remain
environment-only in this release. They are candidates for later vertical
migrations after their services, middleware, workers, and cross-field
validation consume typed snapshots safely.
