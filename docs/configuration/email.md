# Email Delivery & Worker System

Authara includes a built-in email system for verification, account-security,
and organization notifications. Core queues messages for:

- signup, password-reset, and email-change codes;
- account creation, new sign-ins, account disablement, and re-enablement;
- authentication-method additions and removals, password changes, and completed
  email changes (to both the old and new addresses);
- admin-access changes;
- organization invitations, accepted or revoked invitations, membership and
  role changes, ownership transfers, and organization deletion.

Organization invitation emails always contain both the hosted invitation link
and the raw invitation code. Deployments upgrading from versions that exposed
`AUTHARA_INVITATION_EMAIL_INCLUDE_CODE` should remove that setting; invitation
codes are no longer conditionally omitted.

The system is designed to be:

- reliable
- asynchronous
- retry-safe

---

## Overview

Email delivery in Authara is handled via a **job-based worker system**.

Instead of sending emails directly during a request:

1. an email job is created
2. the job is stored in the database
3. background workers process the job
4. the email is sent via the configured provider

Operators can customize each transactional email from the operator workspace.
The worker resolves the currently saved template when it processes a job, so
changes apply without restarting Core. A job that was queued before a template
change uses the latest saved template when it is delivered. Restoring a
template immediately returns subsequent deliveries to the built-in version.
See [Operator email templates](../operations/operator-email-templates.md) for
provisioning, history, audit, backup, and recovery guidance.

Operators can also disable or re-enable each email type from the template
catalog. Every type is enabled by default. A disabled type does not create new
`email_jobs`; jobs already queued before it was disabled are unaffected. There
are no per-notification environment toggles. When enabled, a new-sign-in
notification is created for every new authenticated session and includes the
observed client IP address and user agent when available.

Account deletion intentionally does not queue an email. The deletion flow
removes queued jobs and other direct references to the user's email address, so
queuing a final message would conflict with that privacy boundary.

---

## How it works

### Step 1 — Job creation

When an email needs to be sent:

- a record is inserted into `email_jobs`
- status is set to `pending`

---

### Step 2 — Worker processing

Workers continuously:

- poll for pending jobs
- attempt delivery
- update job status

---

### Step 3 — Delivery result

Depending on the outcome:

- `sent` → success
- `failed` → retry scheduled
- `permanent failure` → job stops retrying

---

## Providers

### noop (default)

```
AUTHARA_EMAIL_PROVIDER=noop
```

- no emails are sent
- the emails logged
- useful for development

---

### smtp

```
AUTHARA_EMAIL_PROVIDER=smtp
```

Uses an SMTP server (e.g. Mailgun, Mailjet, SES).

---

## SMTP configuration

### AUTHARA_EMAIL_FROM

Sender email address.

Example:

```
no-reply@mg.example.com
```

Required when using SMTP.

---

### AUTHARA_EMAIL_SMTP_HOST

SMTP server host.

Example:

```
smtp.mailgun.org
```

---

### AUTHARA_EMAIL_SMTP_PORT

Default:

```
587
```

---

### AUTHARA_EMAIL_SMTP_USERNAME

SMTP username.

---

### AUTHARA_EMAIL_SMTP_PASSWORD

SMTP password.

---

### AUTHARA_EMAIL_SMTP_TLS

Enable TLS.

Default:

```
true
```

---

### AUTHARA_EMAIL_SMTP_TIMEOUT

Timeout for SMTP operations.

Default:

```
10s
```

---

## Worker configuration

### AUTHARA_EMAIL_WORKER_COUNT

Number of concurrent workers.

Default:

```
2
```

Higher values:

- increase throughput
- increase load on SMTP provider

---

### AUTHARA_EMAIL_WORKER_POLL_INTERVAL

How often workers check for new jobs.

Default:

```
2s
```

---

### AUTHARA_EMAIL_JOB_MAX_ATTEMPTS

Maximum retry attempts per job.

Default:

```
10
```

If the environment variable is absent, an operator can change this value at
runtime. The new limit is used when the next failed attempt is evaluated,
including for jobs that are already queued.

---

## Cleanup

Authara automatically cleans up old email records.

### AUTHARA_EMAIL_CLEANUP_SENT_AFTER

Delete successfully sent emails after:

```
720h (30 days)
```

If the environment variable is absent, an operator can change this retention
at runtime. The next cleanup run uses the new cutoff.

---

### AUTHARA_EMAIL_CLEANUP_FAILED_AFTER

Delete failed emails after:

```
2160h (90 days)
```

If the environment variable is absent, an operator can change this retention
at runtime. The next cleanup run uses the new cutoff.

---

## Failure handling

### Temporary failure

Examples:

- SMTP timeout
- provider unavailable

Behavior:

- job is retried
- next attempt is scheduled

---

### Permanent failure

Examples:

- invalid domain
- rejected recipient

Behavior:

- job is marked as failed
- no further retries

---

## Common issues

### Emails not arriving

Check:

- DNS configuration (SPF, DKIM, MX)
- sender domain validity
- SMTP credentials

---

### Emails marked as spam

Improve:

- SPF/DKIM alignment
- DMARC policy
- domain reputation

---

### Provider rejection

Example error:

```
553 domain does not exist
```

Cause:

- missing DNS records (A/MX)

---

## Development setup

For local development:

### Mailpit

Run:

```
docker run -p 1025:1025 -p 8025:8025 axllent/mailpit:v1.31.1
```

Config:

```
AUTHARA_EMAIL_PROVIDER=smtp
AUTHARA_EMAIL_SMTP_HOST=localhost
AUTHARA_EMAIL_SMTP_PORT=1025
AUTHARA_EMAIL_FROM=test@test.com
```

Then open:

```
http://localhost:8025
```
