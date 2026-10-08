import React, { useCallback, useEffect, useRef, useState } from "react";
import { createRoot } from "react-dom/client";

import {
  APIError,
  completeAccountRecoveryLinkWithApple,
  completeAccountRecoveryLinkWithGoogle,
  completeAccountRecoveryLinkWithPassword,
  createOrganization,
  deleteOrganization,
  getAccount,
  getGoogleOptions,
  inviteMember,
  isRecentAuthenticationRequired,
  loadDashboard,
  login,
  loginWithApple,
  loginWithGoogleOrStartRecovery,
  logout,
  refreshSession,
  removeOrganizationMember,
  resendSignupChallenge,
  resendInvitation,
  revokeInvitation,
  signupDirect,
  startSignupChallenge,
  switchOrganization,
  transferOrganizationOwnership,
  updateOrganization,
  verifySignupChallenge,
} from "./api.js";
import {
  AccountPage,
  ConfirmDialog,
  ReauthenticationDialog,
} from "./account.jsx";
import { loadGoogleIdentity } from "./google.js";
import { authorizeWithApple } from "./apple.js";
import { AppleCredentialButton } from "./apple-button.jsx";
import { authenticateWithPasskey } from "./passkeys.js";
import "./styles.css";

const privatePath = "/spa/private";
const accountPath = "/spa/account";

function currentRoute() {
  return window.location.pathname.startsWith(accountPath)
    ? "account"
    : "dashboard";
}

function formatDate(value) {
  const date = new Date(value);
  return Number.isNaN(date.valueOf())
    ? value
    : new Intl.DateTimeFormat(undefined, {
        dateStyle: "medium",
        timeStyle: "short",
      }).format(date);
}

function GoogleCredentialButton({ busy, onCredential, setBusy, setError }) {
  const buttonRef = useRef(null);
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    let active = true;

    getGoogleOptions()
      .then(async (options) => [options, await loadGoogleIdentity()])
      .then(([options, google]) => {
        if (!active || !buttonRef.current) return;

        google.accounts.id.initialize({
          client_id: options.client_id,
          nonce: options.nonce,
          callback: async ({ credential }) => {
            if (!credential) return;
            setBusy(true);
            setError("");
            try {
              await onCredential(credential, options.nonce);
            } catch (error) {
              setError(error.message || "Google sign-in failed.");
            } finally {
              setBusy(false);
            }
          },
        });
        google.accounts.id.renderButton(buttonRef.current, {
          type: "standard",
          theme: "outline",
          size: "large",
          text: "continue_with",
          width: Math.min(buttonRef.current.clientWidth, 400),
        });
        setVisible(true);
      })
      .catch((error) => {
        if (active && (!(error instanceof APIError) || error.status !== 404)) {
          setError(error.message || "Google sign-in is unavailable.");
        }
      });

    return () => {
      active = false;
    };
  }, [onCredential, setBusy, setError]);

  return (
    <div
      className={`google-auth ${visible ? "" : "pending"} ${busy ? "disabled" : ""}`}
    >
      <div className="auth-divider" aria-hidden="true">
        <span>or</span>
      </div>
      <div className="google-button" ref={buttonRef} />
    </div>
  );
}

function GoogleLogin({
  busy,
  onAuthenticated,
  onRecoveryRequired,
  setBusy,
  setError,
}) {
  const authenticate = useCallback(
    async (credential, nonce) => {
      const result = await loginWithGoogleOrStartRecovery(credential, nonce);
      if (result.recovery) {
        onRecoveryRequired(result.recovery);
        return;
      }
      await onAuthenticated();
    },
    [onAuthenticated, onRecoveryRequired],
  );

  return (
    <GoogleCredentialButton
      busy={busy}
      onCredential={authenticate}
      setBusy={setBusy}
      setError={setError}
    />
  );
}

function AccountRecovery({
  busy,
  recovery,
  onAuthenticated,
  onCancel,
  setBusy,
  setError,
}) {
  const [password, setPassword] = useState("");
  const methods = new Set(recovery.proof_methods || []);

  async function complete(action) {
    setBusy(true);
    setError("");
    try {
      await action();
      await onAuthenticated();
    } catch (error) {
      setError(error.message || "Could not verify the existing account.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <p className="eyebrow">Account found</p>
      <h1 id="auth-title">Confirm your existing account.</h1>
      <p className="lede">
        An account already uses this email. Sign in with one of its existing
        methods to connect Google safely.
      </p>

      {methods.has("password") && (
        <form
          className="auth-form recovery-form"
          onSubmit={(event) => {
            event.preventDefault();
            void complete(() =>
              completeAccountRecoveryLinkWithPassword(
                recovery.link_id,
                password,
              ),
            );
          }}
        >
          <label htmlFor="recovery-password">
            <span>Existing account password</span>
            <input
              id="recovery-password"
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              autoComplete="current-password"
              required
              disabled={busy}
            />
          </label>
          <button className="button primary" type="submit" disabled={busy}>
            Continue with password
          </button>
        </form>
      )}

      {methods.has("google") && (
        <GoogleCredentialButton
          busy={busy}
          setBusy={setBusy}
          setError={setError}
          onCredential={(credential, nonce) =>
            completeAccountRecoveryLinkWithGoogle(
              recovery.link_id,
              credential,
              nonce,
            ).then(onAuthenticated)
          }
        />
      )}

      {methods.has("apple") && (
        <AppleCredentialButton
          disabled={busy}
          type="continue"
          onAction={async () => {
            const authorization = await authorizeWithApple();
            await complete(() =>
              completeAccountRecoveryLinkWithApple(
                recovery.link_id,
                authorization.code,
                authorization.state,
              ),
            );
          }}
          onError={setError}
        />
      )}

      {methods.size === 0 && (
        <p className="error" role="alert">
          This account has no available sign-in method for recovery.
        </p>
      )}

      <div className="auth-actions">
        <button
          className="text-button"
          type="button"
          onClick={onCancel}
          disabled={busy}
        >
          Start over
        </button>
      </div>
    </>
  );
}

function AuthScreen({ onAuthenticated }) {
  const [mode, setMode] = useState("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [invitationCode, setInvitationCode] = useState("");
  const [challengeID, setChallengeID] = useState("");
  const [code, setCode] = useState("");
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");
  const [recovery, setRecovery] = useState(null);

  function selectMode(nextMode) {
    setMode(nextMode);
    setPassword("");
    setInvitationCode("");
    setChallengeID("");
    setCode("");
    setNotice("");
    setError("");
    setRecovery(null);
  }

  async function submit(event) {
    event.preventDefault();
    setBusy(true);
    setNotice("");
    setError("");

    try {
      if (mode === "verify") {
        await verifySignupChallenge(challengeID, code);
        await onAuthenticated();
        return;
      }

      if (mode === "login") {
        await login(email, password);
        await onAuthenticated();
        return;
      }

      const signupInvitationCode = mode === "invite" ? invitationCode : "";
      try {
        await signupDirect(email, password, signupInvitationCode);
        await onAuthenticated();
      } catch (signupError) {
        if (!(signupError instanceof APIError) || signupError.status !== 404) {
          throw signupError;
        }
        const challenge = await startSignupChallenge(
          email,
          password,
          signupInvitationCode,
        );
        setChallengeID(challenge.challenge_id);
        setPassword("");
        setMode("verify");
        setNotice("Check your email for the six-digit verification code.");
      }
    } catch (requestError) {
      setError(requestError.message || "Authentication failed.");
    } finally {
      setBusy(false);
    }
  }

  async function resendCode() {
    setBusy(true);
    setNotice("");
    setError("");
    try {
      await resendSignupChallenge(challengeID);
      setNotice("If the challenge is still valid, a new code is on its way.");
    } catch (requestError) {
      setError(requestError.message || "Could not resend the code.");
    } finally {
      setBusy(false);
    }
  }

  const verifying = mode === "verify";

  if (recovery) {
    return (
      <main className="centered">
        <section className="hero auth-card" aria-labelledby="auth-title">
          <AccountRecovery
            busy={busy}
            recovery={recovery}
            onAuthenticated={onAuthenticated}
            onCancel={() => {
              setRecovery(null);
              setError("");
            }}
            setBusy={setBusy}
            setError={setError}
          />
          <div className="auth-feedback" aria-live="polite">
            {error && (
              <p className="error" role="alert">
                {error}
              </p>
            )}
          </div>
        </section>
      </main>
    );
  }

  return (
    <main className="centered">
      <section className="hero auth-card" aria-labelledby="auth-title">
        <p className="eyebrow">Authara browser API</p>
        <h1 id="auth-title">
          {verifying
            ? "Verify your email."
            : mode === "login"
              ? "Welcome back."
              : mode === "invite"
                ? "Join with an invitation."
                : "Create your account."}
        </h1>
        <p className="lede">
          {verifying
            ? `Enter the code sent for ${email}.`
            : "This custom SPA form talks directly to Authara and keeps the session in secure cookies."}
        </p>

        {!verifying && (
          <div className="auth-switch" aria-label="Authentication method">
            <button
              className={mode === "login" ? "active" : ""}
              type="button"
              aria-pressed={mode === "login"}
              onClick={() => selectMode("login")}
              disabled={busy}
            >
              Sign in
            </button>
            <button
              className={mode === "signup" ? "active" : ""}
              type="button"
              aria-pressed={mode === "signup"}
              onClick={() => selectMode("signup")}
              disabled={busy}
            >
              Sign up
            </button>
            <button
              className={mode === "invite" ? "active" : ""}
              type="button"
              aria-pressed={mode === "invite"}
              onClick={() => selectMode("invite")}
              disabled={busy}
            >
              Invite
            </button>
          </div>
        )}

        <form className="auth-form" onSubmit={submit}>
          {verifying ? (
            <label htmlFor="verification-code">
              <span>Verification code</span>
              <input
                id="verification-code"
                value={code}
                onChange={(event) => setCode(event.target.value)}
                inputMode="numeric"
                autoComplete="one-time-code"
                pattern="[0-9]{6}"
                maxLength={6}
                placeholder="123456"
                required
                autoFocus
                disabled={busy}
              />
            </label>
          ) : (
            <>
              <label htmlFor="auth-email">
                <span>Email</span>
                <input
                  id="auth-email"
                  type="email"
                  value={email}
                  onChange={(event) => setEmail(event.target.value)}
                  autoComplete="email"
                  placeholder="you@example.com"
                  required
                  autoFocus
                  disabled={busy}
                />
              </label>
              <label htmlFor="auth-password">
                <span>Password</span>
                <input
                  id="auth-password"
                  type="password"
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  autoComplete={
                    mode === "login" ? "current-password" : "new-password"
                  }
                  minLength={8}
                  maxLength={128}
                  required
                  disabled={busy}
                />
              </label>
              {mode === "invite" && (
                <label htmlFor="invitation-code">
                  <span>Invitation code</span>
                  <input
                    id="invitation-code"
                    value={invitationCode}
                    onChange={(event) => setInvitationCode(event.target.value)}
                    autoComplete="one-time-code"
                    spellCheck="false"
                    className="mono-input"
                    required
                    disabled={busy}
                  />
                </label>
              )}
            </>
          )}

          <button className="button primary" type="submit" disabled={busy}>
            {busy
              ? "Please wait…"
              : verifying
                ? "Verify and continue"
                : mode === "login"
                  ? "Sign in"
                  : mode === "invite"
                    ? "Join organization"
                    : "Create account"}
          </button>
        </form>

        {!verifying && (
          <>
            {mode === "login" && (
              <button
                className="button secondary passkey-login"
                type="button"
                disabled={busy}
                onClick={async () => {
                  setBusy(true);
                  setError("");
                  try {
                    await authenticateWithPasskey();
                    await onAuthenticated();
                  } catch (requestError) {
                    setError(
                      requestError.message || "Passkey sign-in failed.",
                    );
                  } finally {
                    setBusy(false);
                  }
                }}
              >
                Sign in with a passkey
              </button>
            )}
            <GoogleLogin
              busy={busy}
              onAuthenticated={onAuthenticated}
              onRecoveryRequired={setRecovery}
              setBusy={setBusy}
              setError={setError}
            />
            {mode !== "invite" && (
              <AppleCredentialButton
                disabled={busy}
                type={mode === "signup" ? "sign-up" : "sign-in"}
                onAction={async () => {
                  setBusy(true);
                  setError("");
                  try {
                    const authorization = await authorizeWithApple();
                    await loginWithApple(
                      authorization.code,
                      authorization.state,
                    );
                    await onAuthenticated();
                  } finally {
                    setBusy(false);
                  }
                }}
                onError={setError}
              />
            )}
          </>
        )}

        {verifying && (
          <div className="auth-actions">
            <button
              className="text-button"
              type="button"
              onClick={resendCode}
              disabled={busy}
            >
              Resend code
            </button>
            <button
              className="text-button"
              type="button"
              onClick={() => selectMode("signup")}
              disabled={busy}
            >
              Use another email
            </button>
          </div>
        )}

        <div className="auth-feedback" aria-live="polite">
          {notice && <p>{notice}</p>}
          {error && (
            <p className="error" role="alert">
              {error}
            </p>
          )}
        </div>
      </section>
    </main>
  );
}

function ErrorView({ message, onRetry }) {
  return (
    <main className="centered">
      <section className="hero" aria-labelledby="error-title">
        <p className="eyebrow">Authara SPA</p>
        <h1 id="error-title">The private page could not be loaded.</h1>
        <p className="error" role="alert">
          {message}
        </p>
        <button className="button primary" type="button" onClick={onRetry}>
          Try again
        </button>
      </section>
    </main>
  );
}

function Dashboard({
  data,
  busy,
  feedback,
  onRefresh,
  onSwitch,
  onCreateOrganization,
  onDeleteOrganization,
  onUpdateOrganization,
  onInvite,
  onRemoveMember,
  onTransferOwnership,
  onRevokeInvitation,
  onResendInvitation,
  onManageAccount,
  onLogout,
}) {
  const [inviteEmail, setInviteEmail] = useState("");
  const [inviteRole, setInviteRole] = useState("member");
  const [newOrganizationName, setNewOrganizationName] = useState("");
  const [organizationName, setOrganizationName] = useState(
    data.currentOrganization.name,
  );
  const [confirmation, setConfirmation] = useState(null);
  const {
    user,
    organizations,
    currentOrganization,
    members,
    currentMember,
    invitations,
    capabilities,
  } = data;
  const canManageOrganization =
    capabilities.allows_public_organization_management &&
    ["owner", "admin"].includes(currentOrganization.role);
  const canInvite =
    canManageOrganization &&
    capabilities.allows_invitations &&
    invitations !== null;
  const canCreateOrganization =
    capabilities.allows_public_organization_management &&
    capabilities.allows_user_created_team_orgs;

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
    <>
      <main className="shell">
      <header className="topbar">
        <div>
          <p className="eyebrow">Private page</p>
          <h1>Welcome, {user.username || user.email}</h1>
        </div>
        <div className="actions">
          <button
            className="button secondary"
            type="button"
            onClick={onRefresh}
            disabled={busy}
          >
            Refresh session
          </button>
          <button
            className="button quiet"
            type="button"
            onClick={onLogout}
            disabled={busy}
          >
            Log out
          </button>
        </div>
      </header>

      <p
        className={`status ${feedback?.kind === "error" ? "error" : ""}`}
        aria-live="polite"
      >
        {busy ||
          feedback?.message ||
          "All data below comes from Authara's browser APIs."}
      </p>

      <div className="grid">
        <section className="card" aria-labelledby="profile-title">
          <div className="section-heading">
            <div>
              <p className="eyebrow">Identity</p>
              <h2 id="profile-title">Your profile</h2>
            </div>
            <button
              className="text-link"
              type="button"
              onClick={onManageAccount}
            >
              Manage account
            </button>
          </div>
          <dl className="details">
            <div>
              <dt>Email</dt>
              <dd>{user.email}</dd>
            </div>
            <div>
              <dt>Username</dt>
              <dd>{user.username || "Not set"}</dd>
            </div>
            <div>
              <dt>User ID</dt>
              <dd className="mono">{user.id}</dd>
            </div>
            <div>
              <dt>Created</dt>
              <dd>{formatDate(user.created_at)}</dd>
            </div>
            <div>
              <dt>Platform roles</dt>
              <dd>{user.roles?.length ? user.roles.join(", ") : "None"}</dd>
            </div>
          </dl>
        </section>

        <section className="card accent" aria-labelledby="current-org-title">
          <p className="eyebrow">Active context</p>
          <h2 id="current-org-title">{currentOrganization.name}</h2>
          <p className="large-copy">
            You are a {currentOrganization.role} in this organization.
          </p>
          <p className="mono subdued">{currentOrganization.id}</p>
          {canManageOrganization && (
            <>
              <form
                className="management-form accent-form"
                onSubmit={(event) => {
                  event.preventDefault();
                  void onUpdateOrganization(organizationName);
                }}
              >
                <label htmlFor="organization-name">
                  <span>Organization name</span>
                  <input
                    id="organization-name"
                    value={organizationName}
                    onChange={(event) =>
                      setOrganizationName(event.target.value)
                    }
                    required
                  />
                </label>
                <button
                  className="button secondary"
                  type="submit"
                  disabled={
                    busy || organizationName === currentOrganization.name
                  }
                >
                  Update
                </button>
              </form>
              {currentOrganization.role === "owner" &&
                currentOrganization.kind !== "personal" && (
                  <button
                    className="text-button danger-link"
                    type="button"
                    disabled={busy}
                    onClick={() =>
                      confirm({
                        title: "Delete organization?",
                        body: `Permanently delete ${currentOrganization.name}? Authara will reject this while other members remain.`,
                        label: "Delete organization",
                        action: onDeleteOrganization,
                      })
                    }
                  >
                    Delete organization
                  </button>
                )}
            </>
          )}
        </section>

        <section className="card full" aria-labelledby="organizations-title">
          <div className="section-heading">
            <div>
              <p className="eyebrow">Memberships</p>
              <h2 id="organizations-title">Your organizations</h2>
            </div>
            <span className="count">{organizations.length}</span>
          </div>
          <ul className="organization-list">
            {organizations.map((organization) => {
              const active = organization.id === currentOrganization.id;
              return (
                <li key={organization.id}>
                  <div>
                    <strong>{organization.name}</strong>
                    <span>{organization.role}</span>
                  </div>
                  <button
                    className="button small secondary"
                    type="button"
                    onClick={() => onSwitch(organization.id)}
                    disabled={busy || active}
                  >
                    {active ? "Active" : "Switch"}
                  </button>
                </li>
              );
            })}
          </ul>
          {canCreateOrganization && (
            <form
              className="management-form"
              onSubmit={(event) => {
                event.preventDefault();
                void onCreateOrganization(newOrganizationName);
                setNewOrganizationName("");
              }}
            >
              <label htmlFor="new-organization-name">
                <span>Create a team organization</span>
                <input
                  id="new-organization-name"
                  value={newOrganizationName}
                  onChange={(event) =>
                    setNewOrganizationName(event.target.value)
                  }
                  placeholder="Acme team"
                  required
                />
              </label>
              <button className="button primary" type="submit" disabled={busy}>
                Create organization
              </button>
            </form>
          )}
        </section>

        <section className="card full" aria-labelledby="members-title">
          <div className="section-heading">
            <div>
              <p className="eyebrow">Current organization</p>
              <h2 id="members-title">Members</h2>
            </div>
            {members && <span className="count">{members.length}</span>}
          </div>
          {currentMember && (
            <p className="route-result">
              Member detail: <strong>{currentMember.email}</strong> ·{" "}
              {currentMember.role}
            </p>
          )}
          {members === null ? (
            <p className="subdued">
              Member listing is unavailable in the current organization mode.
            </p>
          ) : members.length === 0 ? (
            <p className="subdued">No members were returned.</p>
          ) : (
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th scope="col">Member</th>
                    <th scope="col">Role</th>
                    <th scope="col">Joined</th>
                    <th scope="col">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {members.map((member) => (
                    <tr key={member.user_id}>
                      <td>
                        <strong>{member.username || member.email}</strong>
                        <span>{member.email}</span>
                      </td>
                      <td>{member.role}</td>
                      <td>{formatDate(member.created_at)}</td>
                      <td>
                        <div className="inline-actions">
                          {currentOrganization.role === "owner" &&
                            member.user_id !== user.id &&
                            member.role !== "owner" && (
                              <button
                                className="button small secondary"
                                type="button"
                                disabled={busy}
                                onClick={() =>
                                  confirm({
                                    title: "Transfer ownership?",
                                    body: `Make ${member.email} the owner? You will become an admin.`,
                                    label: "Transfer ownership",
                                    action: () =>
                                      onTransferOwnership(member.user_id),
                                  })
                                }
                              >
                                Make owner
                              </button>
                            )}
                          {canManageOrganization &&
                            member.user_id !== user.id && (
                              <button
                                className="button small quiet"
                                type="button"
                                disabled={busy}
                                onClick={() =>
                                  confirm({
                                    title: "Remove member?",
                                    body: `Remove ${member.email} from ${currentOrganization.name}?`,
                                    label: "Remove member",
                                    action: () =>
                                      onRemoveMember(member.user_id),
                                  })
                                }
                              >
                                Remove
                              </button>
                            )}
                          {member.user_id === user.id &&
                            capabilities.allows_organization_leave &&
                            currentOrganization.kind !== "personal" && (
                              <button
                                className="button small quiet"
                                type="button"
                                disabled={busy}
                                onClick={() =>
                                  confirm({
                                    title: "Leave organization?",
                                    body: `Leave ${currentOrganization.name}?`,
                                    label: "Leave organization",
                                    action: () => onRemoveMember(user.id),
                                  })
                                }
                              >
                                Leave
                              </button>
                            )}
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>

        <section className="card full" aria-labelledby="invitations-title">
          <div className="section-heading">
            <div>
              <p className="eyebrow">Current organization</p>
              <h2 id="invitations-title">Invitations</h2>
            </div>
            {invitations && <span className="count">{invitations.length}</span>}
          </div>
          {canInvite && (
            <form
              className="management-form"
              onSubmit={(event) => {
                event.preventDefault();
                void onInvite(inviteEmail, inviteRole);
                setInviteEmail("");
              }}
            >
              <label htmlFor="invite-email">
                <span>Invite by email</span>
                <input
                  id="invite-email"
                  type="email"
                  value={inviteEmail}
                  onChange={(event) => setInviteEmail(event.target.value)}
                  placeholder="teammate@example.com"
                  required
                />
              </label>
              <label htmlFor="invite-role">
                <span>Role</span>
                <select
                  id="invite-role"
                  value={inviteRole}
                  onChange={(event) => setInviteRole(event.target.value)}
                  disabled={busy}
                >
                  <option value="member">Member</option>
                  <option value="admin">Admin</option>
                </select>
              </label>
              <button className="button primary" type="submit" disabled={busy}>
                Send invitation
              </button>
            </form>
          )}
          {invitations === null ? (
            <p className="subdued">
              Invitation management requires an owner or admin role.
            </p>
          ) : invitations.length === 0 ? (
            <p className="subdued">No invitations were returned.</p>
          ) : (
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th scope="col">Email</th>
                    <th scope="col">Role</th>
                    <th scope="col">Status</th>
                    <th scope="col">Expires</th>
                    <th scope="col">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {invitations.map((invitation) => (
                    <tr key={invitation.id}>
                      <td>
                        <strong>{invitation.email}</strong>
                        <span className="mono">{invitation.id}</span>
                      </td>
                      <td>{invitation.role}</td>
                      <td>{invitation.status}</td>
                      <td>{formatDate(invitation.expires_at)}</td>
                      <td>
                        <div className="inline-actions">
                          {invitation.status === "pending" && (
                            <button
                              className="button small quiet"
                              type="button"
                              onClick={() => onRevokeInvitation(invitation.id)}
                              disabled={busy}
                            >
                              Revoke
                            </button>
                          )}
                          {["pending", "expired"].includes(
                            invitation.status,
                          ) && (
                            <button
                              className="button small secondary"
                              type="button"
                              onClick={() => onResendInvitation(invitation.id)}
                              disabled={busy}
                            >
                              Resend
                            </button>
                          )}
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>
      </div>
      </main>
      {confirmation && (
        <ConfirmDialog
          confirmation={confirmation}
          busy={Boolean(busy)}
          onClose={() => setConfirmation(null)}
        />
      )}
    </>
  );
}

function App() {
  const [route, setRoute] = useState(currentRoute);
  const [view, setView] = useState({ kind: "loading" });
  const [busy, setBusy] = useState("");
  const [feedback, setFeedback] = useState(null);
  const [recentAuthentication, setRecentAuthentication] = useState(null);

  const load = useCallback(async ({ silent = false } = {}) => {
    if (!silent) setView({ kind: "loading" });
    try {
      const data =
        route === "account" ? await getAccount() : await loadDashboard();
      setView({ kind: "ready", route, data });
    } catch (error) {
      if (error instanceof APIError && error.status === 401) {
        setView({ kind: "signed-out" });
      } else {
        setView({
          kind: "error",
          message: error.message || "Unexpected error",
        });
      }
    }
  }, [route]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    const updateRoute = () => setRoute(currentRoute());
    window.addEventListener("popstate", updateRoute);
    return () => window.removeEventListener("popstate", updateRoute);
  }, []);

  function navigate(path) {
    if (window.location.pathname !== path) window.history.pushState({}, "", path);
    setFeedback(null);
    setRoute(currentRoute());
  }

  async function execute(operation) {
    setBusy(operation.label);
    setFeedback(null);
    try {
      const result = await operation.action();
      if (operation.reload !== false) await load({ silent: true });
      operation.onSuccess?.(result);
      if (operation.success) {
        setFeedback({ kind: "success", message: operation.success });
      }
    } catch (error) {
      if (operation.sensitive && isRecentAuthenticationRequired(error)) {
        let account = view.route === "account" ? view.data : null;
        if (!account) {
          try {
            account = await getAccount();
          } catch (accountError) {
            setFeedback({
              kind: "error",
              message:
                accountError.message ||
                "Could not load authentication methods.",
            });
            return;
          }
        }
        setRecentAuthentication({
          challenge: error.authenticationChallenge,
          operation,
          account,
        });
        return;
      }
      if (error instanceof APIError && error.status === 401) {
        setView({ kind: "signed-out" });
      } else {
        setFeedback({
          kind: "error",
          message: error.message || "Unexpected error",
        });
      }
    } finally {
      setBusy("");
    }
  }

  function run(label, action, success = "", onSuccess = null, reload = true) {
    return execute({ label, action, success, onSuccess, reload });
  }

  function runSensitive(
    label,
    action,
    success = "",
    onSuccess = null,
    reload = true,
  ) {
    return execute({
      label,
      action,
      success,
      onSuccess,
      reload,
      sensitive: true,
    });
  }

  async function finishRecentAuthentication() {
    const operation = recentAuthentication?.operation;
    setRecentAuthentication(null);
    if (operation) await execute(operation);
  }

  if (view.kind === "loading") {
    return (
      <main className="centered" aria-live="polite">
        <p>Checking your Authara session…</p>
      </main>
    );
  }
  if (view.kind === "signed-out")
    return <AuthScreen onAuthenticated={() => load()} />;
  if (view.kind === "error")
    return <ErrorView message={view.message} onRetry={() => load()} />;

  if (view.route === "account") {
    return (
      <>
        <AccountPage
          account={view.data}
          busy={busy}
          feedback={feedback}
          onBack={() => navigate(privatePath)}
          onRun={run}
          onSensitive={runSensitive}
          onAccountDeleted={() => setView({ kind: "signed-out" })}
          onLogout={() =>
            run(
              "Logging out…",
              async () => {
                await logout();
                setView({ kind: "signed-out" });
              },
              "",
              null,
              false,
            )
          }
        />
        {recentAuthentication && (
          <ReauthenticationDialog
            challenge={recentAuthentication.challenge}
            account={view.data}
            onCancel={() => setRecentAuthentication(null)}
            onAuthenticated={finishRecentAuthentication}
          />
        )}
      </>
    );
  }

  return (
    <>
    <Dashboard
      key={`${view.data.currentOrganization.id}:${view.data.currentOrganization.name}`}
      data={view.data}
      busy={busy}
      feedback={feedback}
      onRefresh={() => run("Refreshing session…", refreshSession)}
      onSwitch={(id) =>
        run("Switching organization…", () => switchOrganization(id))
      }
      onCreateOrganization={(name) =>
        runSensitive("Creating organization…", () => createOrganization(name))
      }
      onDeleteOrganization={() =>
        runSensitive("Deleting organization…", () =>
          deleteOrganization(view.data.currentOrganization.id),
        )
      }
      onUpdateOrganization={(name) =>
        runSensitive("Updating organization…", () =>
          updateOrganization(view.data.currentOrganization.id, name),
        )
      }
      onInvite={(email, role) =>
        runSensitive("Sending invitation…", () =>
          inviteMember(view.data.currentOrganization.id, email, role),
        )
      }
      onRemoveMember={(userID) =>
        runSensitive("Removing member…", () =>
          removeOrganizationMember(view.data.currentOrganization.id, userID),
        )
      }
      onTransferOwnership={(userID) =>
        runSensitive("Transferring ownership…", () =>
          transferOrganizationOwnership(view.data.currentOrganization.id, userID),
        )
      }
      onRevokeInvitation={(id) =>
        runSensitive("Revoking invitation…", () =>
          revokeInvitation(view.data.currentOrganization.id, id),
        )
      }
      onResendInvitation={(id) =>
        runSensitive("Resending invitation…", () =>
          resendInvitation(view.data.currentOrganization.id, id),
        )
      }
      onManageAccount={() => navigate(accountPath)}
      onLogout={() =>
        run(
          "Logging out…",
          async () => {
            await logout();
            setView({ kind: "signed-out" });
          },
          "",
          null,
          false,
        )
      }
    />
    {recentAuthentication && (
      <ReauthenticationDialog
        challenge={recentAuthentication.challenge}
        account={recentAuthentication.account}
        onCancel={() => setRecentAuthentication(null)}
        onAuthenticated={finishRecentAuthentication}
      />
    )}
    </>
  );
}

createRoot(document.getElementById("root")).render(<App />);
