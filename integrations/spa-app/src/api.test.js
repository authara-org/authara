import assert from "node:assert/strict";
import { afterEach, test } from "node:test";

import {
  createOrganization,
  getGoogleOptions,
  getUserWithRefresh,
  inviteMember,
  login,
  loginWithGoogle,
  loadDashboard,
  resendSignupChallenge,
  resendInvitation,
  revokeInvitation,
  signupDirect,
  startSignupChallenge,
  updateOrganization,
  verifySignupChallenge,
} from "./api.js";

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
});

function json(body, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

test("dashboard refreshes once and uses every public organization read route", async () => {
  const calls = [];
  let userCalls = 0;
  const user = {
    id: "user-1",
    email: "user@example.com",
    roles: [],
    organization: { id: "org-1", name: "Team", role: "owner" },
  };
  const member = {
    organization_id: "org-1",
    user_id: "user-1",
    email: "user@example.com",
    username: "user",
    role: "owner",
    created_at: "2026-01-01T00:00:00Z",
  };
  const invitation = {
    id: "invitation-1",
    organization_id: "org-1",
    email: "teammate@example.com",
    role: "member",
    status: "pending",
    expires_at: "2026-02-01T00:00:00Z",
  };

  globalThis.fetch = async (url, options = {}) => {
    calls.push(`${options.method || "GET"} ${url}`);
    if (url.endsWith("/user")) {
      userCalls += 1;
      return userCalls === 1
        ? json(
            { error: { code: "unauthorized", message: "Unauthorized" } },
            401,
          )
        : json(user);
    }
    if (url.endsWith("/csrf")) return json({ csrf_token: "csrf-token" });
    if (url.includes("/sessions/refresh"))
      return new Response(null, { status: 200 });
    if (url.endsWith("/api/v1/capabilities")) {
      return json({
        allows_invitations: true,
        allows_public_organization_management: true,
        allows_user_created_team_orgs: true,
      });
    }
    if (url.endsWith("/organizations/org-1")) {
      return json({ organization: { id: "org-1", name: "Team" } });
    }
    if (url.endsWith("/organizations/org-1/members")) {
      return json({ members: [member] });
    }
    if (url.endsWith("/organizations/org-1/members/user-1")) {
      return json({ member });
    }
    if (url.endsWith("/organizations/org-1/invitations")) {
      return json({ invitations: [invitation] });
    }
    if (url.endsWith("/organizations/org-1/invitations/invitation-1")) {
      return json({ invitation });
    }
    if (url.endsWith("/users/user-1/memberships")) {
      return json({
        memberships: [
          {
            organization: { id: "org-1", name: "Team" },
            membership: { role: "owner" },
          },
        ],
      });
    }
    throw new Error(`Unexpected request: ${url}`);
  };

  const dashboard = await loadDashboard();

  assert.equal(dashboard.user.id, "user-1");
  assert.deepEqual(dashboard.members, [member]);
  assert.deepEqual(dashboard.currentMember, member);
  assert.deepEqual(dashboard.invitations, [invitation]);
  assert.deepEqual(dashboard.organizations, [
    { id: "org-1", name: "Team", role: "owner" },
  ]);
  assert.equal(
    dashboard.capabilities.allows_public_organization_management,
    true,
  );
  assert.deepEqual(calls.slice(0, 4), [
    "GET /auth/api/v1/user",
    "GET /auth/api/v1/csrf",
    "POST /auth/api/v1/sessions/refresh?audience=app",
    "GET /auth/api/v1/user",
  ]);
  assert.deepEqual(calls.slice(4), [
    "GET /auth/api/v1/capabilities",
    "GET /auth/api/v1/organizations/org-1",
    "GET /auth/api/v1/organizations/org-1/members",
    "GET /auth/api/v1/organizations/org-1/members/user-1",
    "GET /auth/api/v1/organizations/org-1/invitations",
    "GET /auth/api/v1/users/user-1/memberships",
    "GET /auth/api/v1/organizations/org-1/invitations/invitation-1",
  ]);
  assert.equal(calls.includes("GET /auth/api/v1/organizations/current"), false);
});

test("concurrent authentication checks share one rotating refresh", async () => {
  let csrfCalls = 0;
  let refreshCalls = 0;
  let userCalls = 0;

  globalThis.fetch = async (url) => {
    if (url.endsWith("/user")) {
      userCalls += 1;
      return userCalls <= 2
        ? json(
            { error: { code: "unauthorized", message: "Unauthorized" } },
            401,
          )
        : json({ id: "user-1" });
    }
    if (url.endsWith("/csrf")) {
      csrfCalls += 1;
      return json({ csrf_token: "csrf-token" });
    }
    if (url.includes("/sessions/refresh")) {
      refreshCalls += 1;
      return new Response(null, { status: 200 });
    }
    throw new Error(`Unexpected request: ${url}`);
  };

  await Promise.all([getUserWithRefresh(), getUserWithRefresh()]);

  assert.equal(csrfCalls, 1);
  assert.equal(refreshCalls, 1);
});

test("organization mutations use every public route with CSRF", async () => {
  const calls = [];

  globalThis.fetch = async (url, options = {}) => {
    calls.push({ url, options });
    if (url.endsWith("/csrf")) return json({ csrf_token: "csrf-token" });
    return json({}, options.method === "POST" ? 201 : 200);
  };

  await createOrganization("New team");
  await updateOrganization("org/1", "Renamed team");
  await inviteMember("org/1", "teammate@example.com");
  await revokeInvitation("org/1", "invitation/1");
  await resendInvitation("org/1", "invitation/1");

  const mutations = calls.filter(({ url }) => !url.endsWith("/csrf"));
  assert.deepEqual(
    mutations.map(({ url, options }) => `${options.method} ${url}`),
    [
      "POST /auth/api/v1/organizations",
      "PATCH /auth/api/v1/organizations/org%2F1",
      "POST /auth/api/v1/organizations/org%2F1/invitations",
      "POST /auth/api/v1/organizations/org%2F1/invitations/invitation%2F1/revoke",
      "POST /auth/api/v1/organizations/org%2F1/invitations/invitation%2F1/resend",
    ],
  );
  assert.ok(
    mutations.every(
      ({ options }) => options.headers["X-CSRF-Token"] === "csrf-token",
    ),
  );
  assert.deepEqual(JSON.parse(mutations[0].options.body), {
    name: "New team",
  });
  assert.deepEqual(JSON.parse(mutations[1].options.body), {
    name: "Renamed team",
  });
  assert.deepEqual(JSON.parse(mutations[2].options.body), {
    email: "teammate@example.com",
  });
  assert.equal(mutations[3].options.body, undefined);
  assert.equal(mutations[4].options.body, undefined);
});

test("custom authentication uses CSRF and the signup challenge API", async () => {
  const calls = [];

  globalThis.fetch = async (url, options = {}) => {
    calls.push({ url, options });
    if (url.endsWith("/csrf")) return json({ csrf_token: "csrf-token" });
    if (url.includes("/signup/challenges?"))
      return json({ challenge_id: "challenge-1" }, 202);
    if (url.endsWith("/challenges/resend"))
      return new Response(null, { status: 204 });
    return json({ user: { id: "user-1" } });
  };

  await login("user@example.com", "password123");
  await signupDirect("direct@example.com", "password123");
  const started = await startSignupChallenge(
    "new@example.com",
    "password123",
    "invite-code",
  );
  await resendSignupChallenge(started.challenge_id);
  await verifySignupChallenge(started.challenge_id, "123456");

  const mutations = calls.filter(({ url }) => !url.endsWith("/csrf"));
  assert.deepEqual(
    mutations.map(({ url, options }) => `${options.method} ${url}`),
    [
      "POST /auth/api/v1/login?audience=app",
      "POST /auth/api/v1/signup/direct?audience=app",
      "POST /auth/api/v1/signup/challenges?audience=app",
      "POST /auth/api/v1/challenges/resend",
      "POST /auth/api/v1/signup/challenges/verify?audience=app",
    ],
  );
  assert.ok(
    mutations.every(
      ({ options }) => options.headers["X-CSRF-Token"] === "csrf-token",
    ),
  );
  assert.deepEqual(JSON.parse(mutations[0].options.body), {
    identifier: "user@example.com",
    password: "password123",
  });
  assert.deepEqual(JSON.parse(mutations[1].options.body), {
    email: "direct@example.com",
    password: "password123",
  });
  assert.deepEqual(JSON.parse(mutations[2].options.body), {
    email: "new@example.com",
    invitation_code: "invite-code",
    password: "password123",
  });
  assert.deepEqual(JSON.parse(mutations[3].options.body), {
    challenge_id: "challenge-1",
  });
  assert.deepEqual(JSON.parse(mutations[4].options.body), {
    challenge_id: "challenge-1",
    code: "123456",
  });
});

test("Google authentication initializes a nonce and exchanges the credential", async () => {
  const calls = [];

  globalThis.fetch = async (url, options = {}) => {
    calls.push({ url, options });
    if (url.endsWith("/oauth/google/options")) {
      return json({ client_id: "google-client", nonce: "google-nonce" });
    }
    if (url.endsWith("/csrf")) return json({ csrf_token: "csrf-token" });
    return json({ user: { id: "user-1" } });
  };

  const options = await getGoogleOptions();
  await loginWithGoogle("google-id-token", options.nonce);

  assert.deepEqual(
    calls.map(({ url, options: requestOptions }) =>
      `${requestOptions.method || "GET"} ${url}`,
    ),
    [
      "GET /auth/api/v1/oauth/google/options",
      "GET /auth/api/v1/csrf",
      "POST /auth/api/v1/oauth/google?audience=app",
    ],
  );
  assert.equal(calls[2].options.headers["X-CSRF-Token"], "csrf-token");
  assert.deepEqual(JSON.parse(calls[2].options.body), {
    credential: "google-id-token",
    nonce: "google-nonce",
  });
});
