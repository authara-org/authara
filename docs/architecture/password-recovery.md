# Password Recovery Security

Password recovery crosses an account-ownership boundary: an unauthenticated
request can ultimately replace a sign-in credential and revoke active sessions.
Authara therefore treats email verification as authority to rotate an existing
password, not as authority to add a new sign-in method.

## Recovery policy

- Password accounts receive a short-lived, one-time reset code.
- OAuth-only and passkey-only accounts do not receive a password-reset code.
  They sign in with an existing provider or passkey and add a password from
  account settings if desired.
- If every configured sign-in method is unavailable, recovery is handled by the
  deployment operator's account-recovery process. Password reset does not
  silently bypass or replace those methods.
- Unknown and passwordless addresses receive the same public response shape and
  resend behavior as an eligible address. This prevents account and sign-in
  method enumeration.

## Security invariants

1. A reset challenge is bound to the immutable user ID and the user's current
   normalized email address.
2. The account must have a password-provider row with a non-empty password hash
   both when the challenge is created and when it is completed.
3. Password reset never inserts an authentication provider. Adding a password
   requires an authenticated account-settings session.
4. Verification, eligibility revalidation, password mutation, session
   revocation, pending-reset invalidation, notification queueing, and challenge
   consumption occur in one database transaction. If a database operation
   fails, the code remains unconsumed and the credential is unchanged.
5. Completion locks the user before revalidating the pending action and password
   provider, serializing it with authenticated auth-method mutations.
6. Every authenticated password add, change, removal, or replacement invalidates
   all pending password resets for that user. A reset completion also invalidates
   every other pending reset, so an older code cannot overwrite a newer
   credential or regain authority over a newly created password provider.

## Threats and controls

| Threat | Control |
| --- | --- |
| Mailbox control adds a password to an OAuth/passkey-only account | Reset only updates an existing password provider. |
| Public responses reveal whether an account or password method exists | Unknown and ineligible requests use opaque challenges and generic resend responses. |
| A provider is removed after the code is issued | Completion rechecks eligibility while holding the auth-method mutation lock. |
| A provider is removed and later re-added while an old code remains valid | Authenticated password-provider mutations hold the same user lock and invalidate every pending reset. |
| A challenge sent to an old or attacker-controlled address targets another user | Creation and completion require the challenge email to match the user's current email. |
| A completion error burns the one-time code | Mutation runs before challenge consumption in the same database transaction. |
| An older outstanding reset overwrites a newer password | Successful completion deletes all pending resets for the user. |
| A successful reset leaves authenticated sessions usable | Existing sessions and access tokens are revoked before completion commits. |

Access-token revocation can also write a conservative cutoff to the configured
cache. If a later database operation fails, that cutoff may remain even though
the database transaction rolls back; this fails closed by requiring the user to
authenticate again and does not change credentials or consume the reset code.
