# Challenge & Verification

Authara includes a built-in **challenge system** used to verify user actions such as signup.

It is primarily used for:

- email verification during signup
- protecting against fake or automated registrations
- enforcing controlled verification flows

---

## Overview

The challenge system introduces a **two-step authentication flow**:

1. User submits signup data (email + password)
2. A verification challenge is created
3. A one-time code is sent via email
4. The user submits the code
5. The signup is completed

---

## How it works

### Step 1 — Challenge creation

When a signup request is received:

- a `challenge` is created
- a corresponding `verification_code` is generated
- a `pending_signup_action` is stored
- an email job is queued

---

### Step 2 — Code delivery

The email worker:

- picks up the job
- sends the verification code to the user
- updates the challenge metadata

---

### Step 3 — Verification

When the user submits the code:

- the code is validated
- expiration is checked
- attempt limits are enforced

If valid:

- the pending signup action is executed
- the user account is created

---

## Configuration

### AUTHARA_CHALLENGE_ENABLED

Enables or disables the challenge system.

Default:

```
false
```

When disabled:

- signup completes immediately
- no email verification is required

Password recovery remains available, and the email worker continues to process
recovery and security notifications independently of this setting.
Session-bound authentication challenges used for recent-authentication step-up
also remain enabled; they do not send or verify email codes.

This setting is environment-only and startup-only.

---

### AUTHARA_CHALLENGE_TTL

Total lifetime of a challenge.

Default:

```
30m
```

After expiration:

- the challenge becomes invalid
- verification is no longer possible

---

### AUTHARA_CHALLENGE_VERIFICATION_CODE_TTL

Lifetime of the verification code.

Default:

```
10m
```

Must be:

- less than or equal to `AUTHARA_CHALLENGE_TTL`

---

### AUTHARA_CHALLENGE_MAX_ATTEMPTS

Maximum number of verification attempts.

Default:

```
5
```

After exceeding:

- the challenge is locked

---

### AUTHARA_CHALLENGE_MAX_RESENDS

Maximum number of resend requests.

Default:

```
3
```

---

### AUTHARA_CHALLENGE_MIN_RESEND_INTERVAL

Minimum time between resend attempts.

Default:

```
30s
```

Prevents:

- email spam
- abuse of resend functionality

### Verification-code hashing keys

`AUTHARA_CHALLENGE_VERIFICATION_CODE_ACTIVE_KEY_ID` selects the HMAC key used
for newly generated email verification codes.
`AUTHARA_CHALLENGE_VERIFICATION_CODE_KEYS` contains that key and any older keys
still needed to validate unexpired codes:

```env
AUTHARA_CHALLENGE_VERIFICATION_CODE_ACTIVE_KEY_ID=challenge-2026-06
AUTHARA_CHALLENGE_VERIFICATION_CODE_KEYS=challenge-2026-06:BASE64_NEW_KEY,challenge-2026-01:BASE64_OLD_KEY
```

Each value must be valid base64 that decodes to at least 32 bytes. Generate a
key with `openssl rand -base64 32`. These keys are startup-only secrets and
must be independent from JWT signing seeds. After changing the active key,
keep old keys configured for at least the verification-code lifetime.

## Runtime overrides

The five challenge lifetime and limit settings above are hybrid. If their
environment variable is absent, an operator can set or clear a runtime
override at `/auth/operator/settings`. An explicitly supplied environment
value locks that setting and takes precedence. `AUTHARA_CHALLENGE_ENABLED` is
not runtime-editable.

Runtime changes affect newly created challenges and newly generated codes.
Existing rows keep their stored expiry and limits. The accepting replica
changes immediately; other replicas normally update on the next two-second
PostgreSQL reconciliation poll. See
[Operator runtime settings](../operations/runtime-settings.md).

---

## Security considerations

The challenge system protects against:

- automated account creation
- invalid email registrations
- brute-force verification attempts

Additional safeguards:

- hashed verification codes
- attempt counters
- resend throttling
- expiration enforcement

---

## Failure modes

### Expired challenge

User sees:

- "This verification code has expired"

Cause:

- challenge TTL exceeded

---

### Too many attempts

User sees:

- "Too many incorrect attempts"

Cause:

- max attempts reached

---

### Invalid code

User sees:

- "The verification code is incorrect"

Cause:

- wrong code submitted
