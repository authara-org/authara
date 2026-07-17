import React, { useCallback, useEffect, useState } from "react";
import { createRoot } from "react-dom/client";

import {
  APIError,
  loadDashboard,
  logout,
  refreshSession,
  switchOrganization,
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

function SignedOut() {
  return (
    <main className="centered">
      <section className="hero" aria-labelledby="welcome-title">
        <p className="eyebrow">Authara API example</p>
        <h1 id="welcome-title">A private page, without an app backend.</h1>
        <p className="lede">
          Sign in through Authara. This React app reads the resulting browser
          session only through the public API.
        </p>
        <div className="actions">
          <a
            className="button primary"
            href={`/auth/login?return_to=${returnTo}`}
          >
            Log in
          </a>
          <a
            className="button secondary"
            href={`/auth/signup?return_to=${returnTo}`}
          >
            Create account
          </a>
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

function Dashboard({ data, busy, onRefresh, onSwitch, onLogout }) {
  const { user, organizations, currentOrganization, members } = data;

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
        {busy || "All data below comes from Authara's public browser API."}
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
        </section>

        <section className="card full" aria-labelledby="members-title">
          <div className="section-heading">
            <div>
              <p className="eyebrow">Current organization</p>
              <h2 id="members-title">Members</h2>
            </div>
            {members && <span className="count">{members.length}</span>}
          </div>
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
  if (view.kind === "signed-out") return <SignedOut />;
  if (view.kind === "error")
    return <ErrorView message={view.message} onRetry={load} />;

  return (
    <Dashboard
      data={view.data}
      busy={busy}
      onRefresh={() => run("Refreshing session…", refreshSession)}
      onSwitch={(id) =>
        run("Switching organization…", () => switchOrganization(id))
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
