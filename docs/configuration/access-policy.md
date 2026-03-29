# Access Policy

Authara supports an optional **email allowlist** to restrict which users may
access the system.

When enabled, only explicitly allowed email addresses are permitted to:

- register new accounts
- authenticate via any provider (password or OAuth)
- create and refresh sessions

This feature is designed for controlled environments such as internal tools,
early-access products, or staged rollouts.

---

## Overview

The access policy operates as a **central enforcement layer** across Authara:

- evaluated during **signup**
- evaluated during **login**
- enforced during **session creation**
- enforced during **session refresh**

This ensures that access restrictions remain consistent even after a user has
already authenticated.

---

## Enabling the allowlist

Enable the feature via configuration:

```
AUTHARA_ACCESS_POLICY_ALLOWLIST_ENABLED=true
```

When disabled, all emails are allowed and no additional checks are performed.

---

## Behavior

### Allowed email

If an email is present in the allowlist:

- signup proceeds normally
- login succeeds if credentials are valid
- sessions can be created and refreshed

### Disallowed email

If an email is not present in the allowlist:

- signup is rejected
- login is rejected
- session refresh is rejected

---

## Use cases

- Internal applications with restricted access
- Private beta or invite-only products
- Gradual rollout of new systems
- Security-sensitive environments requiring strict admission control
