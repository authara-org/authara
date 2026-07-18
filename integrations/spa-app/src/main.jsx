import React, { useCallback, useEffect, useState } from "react";
import { createRoot } from "react-dom/client";

import {
  APIError,
  createOrganization,
  inviteMember,
  loadDashboard,
  login,
  logout,
  refreshSession,
  resendSignupChallenge,
  resendInvitation,
  revokeInvitation,
  signup,
  switchOrganization,
  updateOrganization,
  verifySignup,
} from "./api.js";
import "./styles.css";

const privatePath = "/spa/private";
const returnTo = encodeURIComponent(privatePath);

function formatDate(value) {
  const date = new Date(value);
  return Number.isNaN(date.valueOf())
    ? value
    : new Intl.DateTimeFormat(undefined, {
        dateStyle: "medium",
        timeStyle: "short",
      }).format(date);
}

function AuthScreen({ onAuthenticated }) {
  const [mode, setMode] = useState("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [challengeID, setChallengeID] = useState("");
  const [code, setCode] = useState("");
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");

  function selectMode(nextMode) {
    setMode(nextMode);
    setPassword("");
    setChallengeID("");
    setCode("");
    setNotice("");
    setError("");
  }

  async function submit(event) {
    event.preventDefault();
    setBusy(true);
    setNotice("");
    setError("");

    try {
      if (mode === "verify") {
        await verifySignup(challengeID, code);
        await onAuthenticated();
        return;
      }

      const result =
        mode === "login"
          ? await login(email, password)
          : await signup(email, password);
      if (result?.challenge_id) {
        setChallengeID(result.challenge_id);
        setPassword("");
        setMode("verify");
        setNotice("Check your email for the six-digit verification code.");
        return;
      }

      await onAuthenticated();
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

  return (
    <main className="centered">
      <section className="hero auth-card" aria-labelledby="auth-title">
        <p className="eyebrow">Authara browser API</p>
        <h1 id="auth-title">
          {verifying
            ? "Verify your email."
            : mode === "login"
              ? "Welcome back."
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
              Log in
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
            </>
          )}

          <button className="button primary" type="submit" disabled={busy}>
            {busy
              ? "Please wait…"
              : verifying
                ? "Verify and continue"
                : mode === "login"
                  ? "Log in"
                  : "Create account"}
          </button>
        </form>

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
  onRefresh,
  onSwitch,
  onCreateOrganization,
  onUpdateOrganization,
  onInvite,
  onRevokeInvitation,
  onResendInvitation,
  onLogout,
}) {
  const [inviteEmail, setInviteEmail] = useState("");
  const [newOrganizationName, setNewOrganizationName] = useState("");
  const [organizationName, setOrganizationName] = useState(
    data.currentOrganization.name,
  );
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

  return (
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

      <p className="status" aria-live="polite">
        {busy || "All data below comes from Authara's browser APIs."}
      </p>

      <div className="grid">
        <section className="card" aria-labelledby="profile-title">
          <div className="section-heading">
            <div>
              <p className="eyebrow">Identity</p>
              <h2 id="profile-title">Your profile</h2>
            </div>
            <a
              className="text-link"
              href={`/auth/account?return_to=${returnTo}`}
            >
              Manage account
            </a>
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
                  onChange={(event) => setOrganizationName(event.target.value)}
                  required
                />
              </label>
              <button
                className="button secondary"
                type="submit"
                disabled={busy || organizationName === currentOrganization.name}
              >
                Update
              </button>
            </form>
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
                void onInvite(inviteEmail);
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
  );
}

function App() {
  const [view, setView] = useState({ kind: "loading" });
  const [busy, setBusy] = useState("");

  const load = useCallback(async () => {
    setBusy("Loading fresh Authara data…");
    try {
      setView({ kind: "ready", data: await loadDashboard() });
    } catch (error) {
      if (error instanceof APIError && error.status === 401) {
        setView({ kind: "signed-out" });
      } else {
        setView({
          kind: "error",
          message: error.message || "Unexpected error",
        });
      }
    } finally {
      setBusy("");
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  async function run(label, action, reload = true) {
    setBusy(label);
    try {
      await action();
      if (reload) await load();
    } catch (error) {
      if (error instanceof APIError && error.status === 401) {
        setView({ kind: "signed-out" });
      } else {
        setView({
          kind: "error",
          message: error.message || "Unexpected error",
        });
      }
    } finally {
      setBusy("");
    }
  }

  if (view.kind === "loading") {
    return (
      <main className="centered" aria-live="polite">
        <p>Checking your Authara session…</p>
      </main>
    );
  }
  if (view.kind === "signed-out")
    return <AuthScreen onAuthenticated={load} />;
  if (view.kind === "error")
    return <ErrorView message={view.message} onRetry={load} />;

  return (
    <Dashboard
      key={`${view.data.currentOrganization.id}:${view.data.currentOrganization.name}`}
      data={view.data}
      busy={busy}
      onRefresh={() => run("Refreshing session…", refreshSession)}
      onSwitch={(id) =>
        run("Switching organization…", () => switchOrganization(id))
      }
      onCreateOrganization={(name) =>
        run("Creating organization…", () => createOrganization(name))
      }
      onUpdateOrganization={(name) =>
        run("Updating organization…", () =>
          updateOrganization(view.data.currentOrganization.id, name),
        )
      }
      onInvite={(email) =>
        run("Sending invitation…", () =>
          inviteMember(view.data.currentOrganization.id, email),
        )
      }
      onRevokeInvitation={(id) =>
        run("Revoking invitation…", () =>
          revokeInvitation(view.data.currentOrganization.id, id),
        )
      }
      onResendInvitation={(id) =>
        run("Resending invitation…", () =>
          resendInvitation(view.data.currentOrganization.id, id),
        )
      }
      onLogout={() =>
        run(
          "Logging out…",
          async () => {
            await logout();
            setView({ kind: "signed-out" });
          },
          false,
        )
      }
    />
  );
}

createRoot(document.getElementById("root")).render(<App />);
