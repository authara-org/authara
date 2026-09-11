# Operator email templates

Authara Core lets operators customize the subject, plain-text body, and HTML
body of its transactional emails. Templates are installation-wide. They are
not organization-specific.

The workspace is available at:

```text
/auth/operator/emails
```

Only users with the `authara:operator` platform role can access these pages.
The admin role does not imply operator access.

## Provisioning and recovery

Grant the operator role to an existing user with the Core command:

```bash
authara operator grant --email operator@example.com
```

For the development Compose environment, the equivalent convenience target
is:

```bash
make operator-by-email EMAIL=operator@example.com
```

Remove access with:

```bash
authara operator revoke --email operator@example.com
```

Role removal revokes the user's active sessions. Keep database and cache
access available as part of the deployment recovery procedure so an operator
role can be granted even when the browser workspace is inaccessible.

## Delivery controls

The **Delivery** switch in the template catalog controls whether Core creates
new jobs for that email type. Every email type is enabled by default. Turning
one off takes effect immediately across Core replicas and prevents subsequent
jobs of that type from being inserted into `email_jobs`.

Jobs that were already queued are not cancelled and continue through normal
worker processing. Turning delivery back on allows new jobs to be queued
again. Delivery settings are installation-wide and are independent of whether
the template uses the built-in content or a customization.

Disabling verification emails such as signup, password-reset, or email-change
codes also prevents users from receiving the code required to complete that
flow.

## Saving and live delivery

Saving validates the complete template before it becomes active. Required
placeholders must be present, subjects cannot contain line breaks, and only
the variables declared for that email type are accepted.

The email worker loads the current saved template when it processes a job.
Consequently:

- a save takes effect without restarting Core;
- every Core replica observes the same database-backed value;
- a message queued before a save uses the template active when it is actually
  processed;
- if no customization exists, Core renders its built-in template.

Previewing a draft or a historical version never changes the active template.

## Built-in email types

Every type below has an independently editable subject, plain-text body, and
HTML body. The editor shows the variables accepted by the selected type and
requires all of them to be present before saving.

| Category | Email types |
| --- | --- |
| Verification | Signup code, password-reset code, email-change code |
| Account | Account created, new sign-in, account disabled, account enabled |
| Authentication | Authentication method added, authentication method removed, password changed, email changed (old address), email changed (new address), admin access changed |
| Organization | Invitation, invitation accepted, invitation revoked, membership removed, role changed, ownership transferred, organization deleted |

When their delivery switches are enabled, security and activity notifications
are queued when the corresponding operation commits. They do not have
individual environment switches. A new sign-in email is queued for every
authenticated session and receives the observed IP address and user agent when
available. An email-change completion notifies both addresses so either mailbox
can identify an unauthorized change.

Core does not send an account-deleted email. User deletion deliberately removes
queued jobs and other direct email-address references, and a final outbound job
would retain the address after that cleanup.

## History and recovery

Every successful save appends an immutable snapshot to
`authara.email_template_versions`. Saving a historical version creates a new
latest version; it does not modify the old snapshot.

The **Restore** action removes the current override and immediately makes the
built-in template active. Historical custom versions remain available and can
later be previewed or saved as a new version.

Concurrent changes use revision checks. If another operator saves or restores
the same template first, the stale operation is rejected instead of silently
overwriting the newer value.

## Audit records

Successful saves, restores, and delivery-setting changes are recorded in
`authara.operator_audit_events` in the same database statement as the template
change. Each event contains:

- the operator user ID;
- the action and template key;
- the resulting revision and history version where applicable;
- the timestamp.

Template source, rendered HTML, codes, invitation URLs, and recipient data are
not copied into audit metadata. Failed validation and revision conflicts do not
create audit events because they do not change the template.

Operators can review and filter these events at:

```text
/auth/operator/audit
```

The page exposes only the action, template, operator ID, timestamp, revision,
and version. Admin-only and regular accounts cannot access it.

Audit records remain until removed through normal database-retention
procedures. If the actor user is deleted, the event remains and its actor ID is
set to `NULL`.

## Backup

Email-template state is stored in these tables:

```text
authara.email_template_overrides
authara.email_template_versions
authara.email_template_delivery_settings
authara.operator_audit_events
```

A full PostgreSQL backup is the recommended recovery unit because it preserves
users, foreign-key relationships, templates, history, audit events, and schema
version state together:

```bash
pg_dump --format=custom --file=authara.dump "$DATABASE_URL"
```

Protect the backup as sensitive operational data. Historical templates can
contain branding, links, and text entered by operators even though audit event
metadata does not contain template bodies.

## Restore

Prefer restoring the complete database into an empty database using the same
Authara Core release:

```bash
createdb authara_restore
pg_restore --exit-on-error --dbname=authara_restore authara.dump
```

Then point a non-production Core instance at the restored database and verify:

1. Core accepts the restored schema version.
2. The operator template overview loads.
3. Current and historical templates preview correctly.
4. A test signup email reaches Mailpit or the deployment's test SMTP inbox.

Do not restore only the template-related tables into a running production
database without first resolving existing primary keys, foreign keys, and
current revisions. For a single broken template, using **Restore** in the
operator workspace is safer than a partial database restore because the
built-in template is always available.
