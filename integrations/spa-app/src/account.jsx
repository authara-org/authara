import React, { useEffect, useRef, useState } from "react";

import {
  APIError,
  addPassword,
  changePassword,
  changeUsername,
  deleteCurrentAccount,
  deletePasskey,
  getGoogleOptions,
  linkApple,
  linkGoogle,
  reauthenticateWithApple,
  reauthenticateWithGoogle,
  reauthenticateWithPassword,
  revokeOtherSessions,
  revokeSession,
  startEmailChange,
  unlinkAuthMethod,
  verifyEmailChange,
} from "./api.js";
import { loadGoogleIdentity } from "./google.js";
import { authorizeWithApple } from "./apple.js";
import { AppleCredentialButton } from "./apple-button.jsx";
import {
  reauthenticateWithPasskey,
  registerPasskey,
} from "./passkeys.js";

function formatDate(value) {
  if (!value) return "Never";
  const date = new Date(value);
  return Number.isNaN(date.valueOf())
    ? value
    : new Intl.DateTimeFormat(undefined, {
        dateStyle: "medium",
        timeStyle: "short",
      }).format(date);
}

function GoogleCredentialButton({ disabled, onCredential, onError }) {
  const buttonRef = useRef(null);
  const credentialHandler = useRef(onCredential);
  const errorHandler = useRef(onError);
  const [available, setAvailable] = useState(true);
  const [ready, setReady] = useState(false);

  credentialHandler.current = onCredential;
  errorHandler.current = onError;

  useEffect(() => {
    let active = true;
    getGoogleOptions()
      .then(async (options) => [options, await loadGoogleIdentity()])
      .then(([options, google]) => {
        if (!active || !buttonRef.current) return;
        google.accounts.id.initialize({
          client_id: options.client_id,
          nonce: options.nonce,
          callback: ({ credential }) => {
            if (credential) {
              void credentialHandler.current(credential, options.nonce);
            }
          },
        });
        google.accounts.id.renderButton(buttonRef.current, {
          type: "standard",
          theme: "outline",
          size: "large",
          text: "continue_with",
          width: Math.min(buttonRef.current.clientWidth || 320, 400),
        });
        setReady(true);
      })
      .catch((error) => {
        if (!active) return;
        if (error instanceof APIError && error.status === 404) {
          setAvailable(false);
          return;
        }
        errorHandler.current(
          error.message || "Google authentication is unavailable.",
        );
      });
    return () => {
      active = false;
    };
  }, []);

  if (!available) return <p className="subdued">Google is not enabled.</p>;
  return (
    <div
      className={`google-action ${ready ? "" : "pending"} ${disabled ? "disabled" : ""}`}
      ref={buttonRef}
    />
  );
}

function Dialog({ title, children, onClose, closeLabel = "Cancel" }) {
  useEffect(() => {
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    const closeOnEscape = (event) => {
      if (event.key === "Escape") onClose();
    };
    document.addEventListener("keydown", closeOnEscape);
    return () => {
      document.body.style.overflow = previousOverflow;
      document.removeEventListener("keydown", closeOnEscape);
    };
  }, [onClose]);

  return (
    <div
      className="dialog-backdrop"
      role="presentation"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <section className="dialog" role="dialog" aria-modal="true" aria-labelledby="dialog-title">
        <div className="section-heading">
          <h2 id="dialog-title">{title}</h2>
          <button className="dialog-close" type="button" onClick={onClose} aria-label={closeLabel}>
            ×
          </button>
        </div>
        {children}
      </section>
    </div>
  );
}

export function ReauthenticationDialog({ challenge, account, onCancel, onAuthenticated }) {
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const providers = new Set(account.auth_methods.map((method) => method.provider));

  async function authenticate(action) {
    setBusy(true);
    setError("");
    try {
      await action();
      await onAuthenticated();
    } catch (requestError) {
      setError(requestError.message || "Authentication failed.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog title="Confirm it’s you" onClose={busy ? () => {} : onCancel}>
      <p className="dialog-copy">
        This action changes sign-in or security data. Authenticate again and the SPA will finish the action automatically.
      </p>

      {providers.has("password") && (
        <form
          className="stack-form"
          onSubmit={(event) => {
            event.preventDefault();
            void authenticate(() => reauthenticateWithPassword(challenge.id, password));
          }}
        >
          <label htmlFor="reauth-password">
            <span>Password</span>
            <input
              id="reauth-password"
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              autoComplete="current-password"
              required
              autoFocus
              disabled={busy}
            />
          </label>
          <button className="button primary" type="submit" disabled={busy}>
            {busy ? "Checking…" : "Continue with password"}
          </button>
        </form>
      )}

      {account.passkeys.length > 0 && (
        <button
          className="button secondary wide"
          type="button"
          disabled={busy}
          onClick={() => void authenticate(() => reauthenticateWithPasskey(challenge.id))}
        >
          Continue with a passkey
        </button>
      )}

      {providers.has("google") && (
        <GoogleCredentialButton
          disabled={busy}
          onCredential={(credential, nonce) =>
            authenticate(() => reauthenticateWithGoogle(challenge.id, credential, nonce))
          }
          onError={setError}
        />
      )}

      {providers.has("apple") && (
        <AppleCredentialButton
          disabled={busy}
          onAction={() =>
            authenticate(async () => {
              const authorization = await authorizeWithApple();
              return reauthenticateWithApple(
                challenge.id,
                authorization.code,
                authorization.state,
              );
            })
          }
          onError={setError}
        />
      )}

      {error && <p className="error dialog-feedback" role="alert">{error}</p>}
      <button className="text-button dialog-cancel" type="button" onClick={onCancel} disabled={busy}>
        Cancel
      </button>
    </Dialog>
  );
}

export function ConfirmDialog({ confirmation, busy, onClose }) {
  return (
    <Dialog title={confirmation.title} onClose={busy ? () => {} : onClose}>
      <p className="dialog-copy">{confirmation.body}</p>
      <div className="dialog-actions">
        <button className="button secondary" type="button" onClick={onClose} disabled={busy}>
          Cancel
        </button>
        <button className="button quiet" type="button" onClick={confirmation.action} disabled={busy}>
          {confirmation.label}
        </button>
      </div>
    </Dialog>
  );
}

export function AccountPage({ account, busy, feedback, onBack, onRun, onSensitive, onLogout, onAccountDeleted }) {
  const [username, setUsername] = useState(account.user.username || "");
  const [newEmail, setNewEmail] = useState("");
  const [emailChallengeID, setEmailChallengeID] = useState("");
  const [emailCode, setEmailCode] = useState("");
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [passkeyName, setPasskeyName] = useState("");
  const [confirmation, setConfirmation] = useState(null);
  const [localError, setLocalError] = useState("");
  const providers = new Set(account.auth_methods.map((method) => method.provider));
  const hasPassword = providers.has("password");
  const hasGoogle = providers.has("google");
  const hasApple = providers.has("apple");

  function confirm(config) {
    setConfirmation({
      ...config,
      action: () => {
        setConfirmation(null);
        config.action();
      },
    });
  }

  return (
    <main className="shell account-shell">
      <header className="topbar">
        <div>
          <p className="eyebrow">API-only account page</p>
          <h1>Account security</h1>
        </div>
        <div className="actions">
          <button className="button secondary" type="button" onClick={onBack} disabled={busy}>
            Back to dashboard
          </button>
          <button className="button quiet" type="button" onClick={onLogout} disabled={busy}>
            Log out
          </button>
        </div>
      </header>

      <p
        className={`status ${localError || feedback?.kind === "error" ? "error" : ""}`}
        aria-live="polite"
      >
        {busy ||
          localError ||
          feedback?.message ||
          "Every control on this page calls /auth/api/v1 directly."}
      </p>

      <div className="grid account-grid">
        <section className="card" aria-labelledby="profile-settings-title">
          <p className="eyebrow">Profile</p>
          <h2 id="profile-settings-title">Public details</h2>
          <p className="subdued">{account.user.email}</p>
          <form
            className="stack-form"
            onSubmit={(event) => {
              event.preventDefault();
              void onRun("Updating username…", () => changeUsername(username), "Username updated.");
            }}
          >
            <label htmlFor="account-username">
              <span>Username</span>
              <input id="account-username" value={username} onChange={(event) => setUsername(event.target.value)} required disabled={busy} />
            </label>
            <button className="button primary" type="submit" disabled={busy || username === account.user.username}>
              Save username
            </button>
          </form>
        </section>

        <section className="card" aria-labelledby="email-title">
          <p className="eyebrow">Verified email</p>
          <h2 id="email-title">Change email</h2>
          {!emailChallengeID ? (
            <form
              className="stack-form"
              onSubmit={(event) => {
                event.preventDefault();
                void onSensitive(
                  "Sending verification code…",
                  () => startEmailChange(newEmail),
                  "Verification code sent.",
                  (result) => setEmailChallengeID(result.challenge_id),
                );
              }}
            >
              <label htmlFor="new-email">
                <span>New email</span>
                <input id="new-email" type="email" value={newEmail} onChange={(event) => setNewEmail(event.target.value)} required disabled={busy} />
              </label>
              <button className="button primary" type="submit" disabled={busy}>Send code</button>
            </form>
          ) : (
            <form
              className="stack-form"
              onSubmit={(event) => {
                event.preventDefault();
                void onSensitive(
                  "Verifying email…",
                  () => verifyEmailChange(emailChallengeID, emailCode),
                  "Email changed.",
                  () => {
                    setEmailChallengeID("");
                    setEmailCode("");
                    setNewEmail("");
                  },
                );
              }}
            >
              <p className="subdued">Enter the six-digit code sent to {newEmail}.</p>
              <label htmlFor="email-code">
                <span>Verification code</span>
                <input id="email-code" className="mono-input" inputMode="numeric" autoComplete="one-time-code" pattern="[0-9]{6}" maxLength={6} value={emailCode} onChange={(event) => setEmailCode(event.target.value)} required disabled={busy} />
              </label>
              <button className="button primary" type="submit" disabled={busy}>Verify email</button>
              <button className="text-button" type="button" onClick={() => setEmailChallengeID("")} disabled={busy}>Use another email</button>
            </form>
          )}
        </section>

        <section className="card full" aria-labelledby="methods-title">
          <div className="section-heading">
            <div>
              <p className="eyebrow">Authentication</p>
              <h2 id="methods-title">Sign-in methods</h2>
            </div>
            <span className="count">{account.auth_methods.length + account.passkeys.length}</span>
          </div>

          <div className="method-grid">
            <div className="method-panel">
              <div>
                <strong>Password</strong>
                <p>{hasPassword ? "Linked" : "Not linked"}</p>
              </div>
              {hasPassword ? (
                <>
                  <form
                    className="stack-form"
                    onSubmit={(event) => {
                      event.preventDefault();
                      void onRun(
                        "Changing password…",
                        () => changePassword(currentPassword, newPassword),
                        "Password changed.",
                        () => {
                          setCurrentPassword("");
                          setNewPassword("");
                        },
                      );
                    }}
                  >
                    <label htmlFor="current-password"><span>Current password</span><input id="current-password" type="password" autoComplete="current-password" value={currentPassword} onChange={(event) => setCurrentPassword(event.target.value)} required disabled={busy} /></label>
                    <label htmlFor="new-password"><span>New password</span><input id="new-password" type="password" autoComplete="new-password" minLength={8} maxLength={128} value={newPassword} onChange={(event) => setNewPassword(event.target.value)} required disabled={busy} /></label>
                    <button className="button secondary" type="submit" disabled={busy}>Change password</button>
                  </form>
                  <button
                    className="text-button danger-link"
                    type="button"
                    disabled={busy}
                    onClick={() => confirm({
                      title: "Remove password?",
                      body: "You will no longer be able to sign in with a password.",
                      label: "Remove password",
                      action: () => onSensitive("Removing password…", () => unlinkAuthMethod("password"), "Password removed."),
                    })}
                  >Remove password</button>
                </>
              ) : (
                <form
                  className="stack-form"
                  onSubmit={(event) => {
                    event.preventDefault();
                    void onSensitive("Adding password…", () => addPassword(newPassword), "Password added.", () => setNewPassword(""));
                  }}
                >
                  <label htmlFor="add-password"><span>New password</span><input id="add-password" type="password" autoComplete="new-password" minLength={8} maxLength={128} value={newPassword} onChange={(event) => setNewPassword(event.target.value)} required disabled={busy} /></label>
                  <button className="button secondary" type="submit" disabled={busy}>Add password</button>
                </form>
              )}
            </div>

            <div className="method-panel">
              <div>
                <strong>Google</strong>
                <p>{hasGoogle ? "Linked" : "Not linked"}</p>
              </div>
              {hasGoogle ? (
                <button
                  className="text-button danger-link"
                  type="button"
                  disabled={busy}
                  onClick={() => confirm({
                    title: "Unlink Google?",
                    body: "You will no longer be able to sign in with this Google identity.",
                    label: "Unlink Google",
                    action: () => onSensitive("Unlinking Google…", () => unlinkAuthMethod("google"), "Google unlinked."),
                  })}
                >Unlink Google</button>
              ) : (
                <GoogleCredentialButton
                  disabled={busy}
                  onCredential={(credential, nonce) =>
                    onSensitive("Linking Google…", () => linkGoogle(credential, nonce), "Google linked.")
                  }
                  onError={setLocalError}
                />
              )}
            </div>

            <div className="method-panel">
              <div>
                <strong>Apple</strong>
                <p>{hasApple ? "Linked" : "Not linked"}</p>
              </div>
              {hasApple ? (
                <button
                  className="text-button danger-link"
                  type="button"
                  disabled={busy}
                  onClick={() => confirm({
                    title: "Unlink Apple?",
                    body: "You will no longer be able to sign in with this Apple identity.",
                    label: "Unlink Apple",
                    action: () => onSensitive("Unlinking Apple…", () => unlinkAuthMethod("apple"), "Apple unlinked."),
                  })}
                >Unlink Apple</button>
              ) : (
                <AppleCredentialButton
                  disabled={busy}
                  onAction={() => onSensitive(
                    "Linking Apple…",
                    async () => {
                      const authorization = await authorizeWithApple();
                      return linkApple(authorization.code, authorization.state);
                    },
                    "Apple linked.",
                  )}
                  onError={setLocalError}
                />
              )}
            </div>
          </div>
        </section>

        <section className="card full" aria-labelledby="passkeys-title">
          <div className="section-heading">
            <div><p className="eyebrow">WebAuthn</p><h2 id="passkeys-title">Passkeys</h2></div>
            <span className="count">{account.passkeys.length}</span>
          </div>
          <form
            className="management-form"
            onSubmit={(event) => {
              event.preventDefault();
              void onSensitive("Adding passkey…", () => registerPasskey(passkeyName), "Passkey added.", () => setPasskeyName(""));
            }}
          >
            <label htmlFor="passkey-name"><span>Name this passkey</span><input id="passkey-name" value={passkeyName} onChange={(event) => setPasskeyName(event.target.value)} placeholder="MacBook Touch ID" disabled={busy} /></label>
            <button className="button primary" type="submit" disabled={busy}>Add passkey</button>
          </form>
          {account.passkeys.length === 0 ? <p className="subdued">No passkeys are linked.</p> : (
            <ul className="security-list">
              {account.passkeys.map((passkey) => (
                <li key={passkey.id}>
                  <div><strong>{passkey.name || "Passkey"}</strong><span>Added {formatDate(passkey.created_at)} · Last used {formatDate(passkey.last_used_at)}</span></div>
                  <button className="button small quiet" type="button" disabled={busy} onClick={() => confirm({
                    title: "Delete passkey?",
                    body: `Delete ${passkey.name || "this passkey"} from your account?`,
                    label: "Delete passkey",
                    action: () => onSensitive("Deleting passkey…", () => deletePasskey(passkey.id), "Passkey deleted."),
                  })}>Delete</button>
                </li>
              ))}
            </ul>
          )}
        </section>

        <section className="card full" aria-labelledby="sessions-title">
          <div className="section-heading">
            <div><p className="eyebrow">Access</p><h2 id="sessions-title">Sessions</h2></div>
            <button className="button small quiet" type="button" disabled={busy || account.sessions.every((session) => session.current)} onClick={() => confirm({
              title: "Revoke other sessions?",
              body: "Every session except this browser will be signed out.",
              label: "Revoke other sessions",
              action: () => onRun("Revoking sessions…", revokeOtherSessions, "Other sessions revoked."),
            })}>Revoke all others</button>
          </div>
          <ul className="security-list">
            {account.sessions.map((session) => (
              <li key={session.id}>
                <div><strong>{session.current ? "This browser" : session.user_agent || "Unknown browser"}</strong><span>Created {formatDate(session.created_at)} · Expires {formatDate(session.expires_at)}</span></div>
                {!session.current && <button className="button small quiet" type="button" disabled={busy} onClick={() => confirm({
                  title: "Revoke session?",
                  body: "That browser will need to sign in again.",
                  label: "Revoke session",
                  action: () => onRun("Revoking session…", () => revokeSession(session.id), "Session revoked."),
                })}>Revoke</button>}
              </li>
            ))}
          </ul>
        </section>

        <section className="card full danger-zone" aria-labelledby="delete-account-title">
          <div>
            <p className="eyebrow">Danger zone</p>
            <h2 id="delete-account-title">Delete account</h2>
            <p className="subdued">
              This calls the SPA backend, which resolves your user from the active session and then uses Authara's internal API. The internal token is never exposed to React.
            </p>
          </div>
          <button
            className="button quiet"
            type="button"
            disabled={busy}
            onClick={() =>
              confirm({
                title: "Delete your account?",
                body: "This permanently deletes your Authara user. Organizations may need to be left, deleted, or transferred first.",
                label: "Delete account",
                action: () =>
                  onSensitive(
                    "Deleting account…",
                    deleteCurrentAccount,
                    "",
                    onAccountDeleted,
                    false,
                  ),
              })
            }
          >
            Delete account
          </button>
        </section>
      </div>

      {confirmation && <ConfirmDialog confirmation={confirmation} busy={Boolean(busy)} onClose={() => setConfirmation(null)} />}
    </main>
  );
}
