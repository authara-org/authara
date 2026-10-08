import assert from "node:assert/strict";
import { afterEach, test } from "node:test";

import {
  APIError,
  addPassword,
  beginPasskeyAuthentication,
  beginPasskeyReauthentication,
  beginPasskeyRegistration,
  changePassword,
  changeUsername,
  completeAccountRecoveryLinkWithApple,
  completeAccountRecoveryLinkWithGoogle,
  completeAccountRecoveryLinkWithPassword,
  createOrganization,
  deleteCurrentAccount,
  deleteOrganization,
  deletePasskey,
  finishPasskeyReauthentication,
  finishPasskeyRegistration,
  finishPasskeyAuthentication,
  getAccount,
  getAppleOptions,
  getGoogleOptions,
  getUserWithRefresh,
  inviteMember,
  isRecentAuthenticationRequired,
  linkGoogle,
  linkApple,
  login,
  loginWithGoogle,
  loginWithGoogleOrStartRecovery,
  loginWithApple,
  loadDashboard,
  reauthenticateWithGoogle,
  reauthenticateWithApple,
  reauthenticateWithPassword,
  removeOrganizationMember,
  resendSignupChallenge,
  resendInvitation,
  revokeOtherSessions,
  revokeSession,
  revokeInvitation,
  signupDirect,
  startEmailChange,
  startGoogleAccountRecoveryLink,
  startSignupChallenge,
  transferOrganizationOwnership,
  unlinkAuthMethod,
  updateOrganization,
  verifyEmailChange,
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

test("organization mutations use public routes or the SPA backend with CSRF", async () => {
  const calls = [];

  globalThis.fetch = async (url, options = {}) => {
    calls.push({ url, options });
    if (url.endsWith("/csrf")) return json({ csrf_token: "csrf-token" });
    return json({}, options.method === "POST" ? 201 : 200);
  };

  await createOrganization("New team");
  await updateOrganization("org/1", "Renamed team");
  await inviteMember("org/1", "teammate@example.com", "admin");
  await revokeInvitation("org/1", "invitation/1");
  await resendInvitation("org/1", "invitation/1");
  await deleteOrganization("org/1");
  await removeOrganizationMember("org/1", "user/2");
  await transferOrganizationOwnership("org/1", "user/2");
  await deleteCurrentAccount();

  const mutations = calls.filter(({ url }) => !url.endsWith("/csrf"));
  assert.deepEqual(
    mutations.map(({ url, options }) => `${options.method} ${url}`),
    [
      "POST /spa/api/v1/organizations",
      "PATCH /auth/api/v1/organizations/org%2F1",
      "POST /spa/api/v1/organizations/org%2F1/invitations",
      "POST /auth/api/v1/organizations/org%2F1/invitations/invitation%2F1/revoke",
      "POST /spa/api/v1/organizations/org%2F1/invitations/invitation%2F1/resend",
      "DELETE /spa/api/v1/organizations/org%2F1",
      "DELETE /spa/api/v1/organizations/org%2F1/members/user%2F2",
      "POST /spa/api/v1/organizations/org%2F1/ownership-transfer",
      "DELETE /spa/api/v1/account",
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
    role: "admin",
  });
  assert.equal(mutations[3].options.body, undefined);
  assert.equal(mutations[4].options.body, undefined);
  assert.equal(mutations[5].options.body, undefined);
  assert.equal(mutations[6].options.body, undefined);
  assert.deepEqual(JSON.parse(mutations[7].options.body), {
    new_owner_user_id: "user/2",
  });
  assert.equal(mutations[8].options.body, undefined);
});

test("SPA backend mutations refresh an expired browser session once", async () => {
  let backendCalls = 0;
  let refreshCalls = 0;

  globalThis.fetch = async (url) => {
    if (url.endsWith("/csrf")) return json({ csrf_token: "csrf-token" });
    if (url.includes("/sessions/refresh")) {
      refreshCalls += 1;
      return new Response(null, { status: 200 });
    }
    if (url === "/spa/api/v1/organizations") {
      backendCalls += 1;
      return backendCalls === 1
        ? json(
            { error: { code: "unauthorized", message: "Unauthorized." } },
            401,
          )
        : json({ organization: { id: "org-1" } }, 201);
    }
    throw new Error(`Unexpected request: ${url}`);
  };

  const result = await createOrganization("Example");

  assert.equal(result.organization.id, "org-1");
  assert.equal(backendCalls, 2);
  assert.equal(refreshCalls, 1);
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

test("Apple authentication obtains a server flow and exchanges only code and state", async () => {
  const calls = [];
  globalThis.fetch = async (url, options = {}) => {
    calls.push({ url, options });
    if (url.endsWith("/oauth/apple/options")) {
      return json({
        client_id: "apple-client",
        redirect_uri: "https://auth.example.com/auth/oauth/apple/callback",
        state: "apple-state",
        nonce: "apple-nonce",
      });
    }
    if (url.endsWith("/csrf")) return json({ csrf_token: "csrf-token" });
    return json({ user: { id: "user-1" } });
  };

  const options = await getAppleOptions();
  await loginWithApple("apple-code", options.state);

  assert.deepEqual(
    calls.map(
      ({ url, options: requestOptions }) =>
        `${requestOptions.method || "GET"} ${url}`,
    ),
    [
      "GET /auth/api/v1/oauth/apple/options",
      "GET /auth/api/v1/csrf",
      "POST /auth/api/v1/oauth/apple?audience=app",
    ],
  );
  assert.deepEqual(JSON.parse(calls[2].options.body), {
    code: "apple-code",
    state: "apple-state",
  });
});

test("account collision recovery supports password, Google, and Apple proof", async () => {
  const calls = [];
  globalThis.fetch = async (url, options = {}) => {
    calls.push({ url, options });
    if (url.endsWith("/csrf")) return json({ csrf_token: "csrf-token" });
    if (url.endsWith("/provider-links/recovery/google")) {
      return json(
        {
          link_id: "link-1",
          proof_methods: ["password", "google", "apple"],
        },
        202,
      );
    }
    return json({ user: { id: "user-1" } });
  };

  const recovery = await startGoogleAccountRecoveryLink(
    "new-google-token",
    "google-nonce",
  );
  await completeAccountRecoveryLinkWithPassword(recovery.link_id, "password123");
  await completeAccountRecoveryLinkWithGoogle(
    recovery.link_id,
    "existing-google-token",
    "proof-nonce",
  );
  await completeAccountRecoveryLinkWithApple(
    recovery.link_id,
    "apple-code",
    "apple-state",
  );

  const mutations = calls.filter(({ options }) => options.method === "POST");
  assert.deepEqual(
    mutations.map(({ url }) => url),
    [
      "/auth/api/v1/provider-links/recovery/google",
      "/auth/api/v1/provider-links/recovery/link-1/password?audience=app",
      "/auth/api/v1/provider-links/recovery/link-1/google?audience=app",
      "/auth/api/v1/provider-links/recovery/link-1/apple?audience=app",
    ],
  );
  assert.deepEqual(JSON.parse(mutations[0].options.body), {
    credential: "new-google-token",
    nonce: "google-nonce",
  });
  assert.deepEqual(JSON.parse(mutations[3].options.body), {
    code: "apple-code",
    state: "apple-state",
  });
});

test("Google login turns account_link_required into a recovery flow", async () => {
  const calls = [];
  globalThis.fetch = async (url, options = {}) => {
    calls.push({ url, options });
    if (url.endsWith("/csrf")) return json({ csrf_token: "csrf-token" });
    if (url.includes("/oauth/google?")) {
      return json(
        {
          error: {
            code: "account_link_required",
            message: "Confirm the existing account.",
          },
        },
        409,
      );
    }
    if (url.endsWith("/provider-links/recovery/google")) {
      return json(
        { link_id: "link-1", proof_methods: ["apple"] },
        202,
      );
    }
    throw new Error(`Unexpected request: ${url}`);
  };

  const result = await loginWithGoogleOrStartRecovery(
    "google-token",
    "google-nonce",
  );

  assert.equal(result.session, null);
  assert.deepEqual(result.recovery, {
    link_id: "link-1",
    proof_methods: ["apple"],
  });
  assert.deepEqual(
    calls
      .filter(({ options }) => options.method === "POST")
      .map(({ url, options }) => [url, JSON.parse(options.body)]),
    [
      [
        "/auth/api/v1/oauth/google?audience=app",
        { credential: "google-token", nonce: "google-nonce" },
      ],
      [
        "/auth/api/v1/provider-links/recovery/google",
        { credential: "google-token", nonce: "google-nonce" },
      ],
    ],
  );
});

test("recent-auth errors retain the challenge needed by an API-only client", async () => {
  const calls = [];
  let unlinkAttempts = 0;

  globalThis.fetch = async (url, options = {}) => {
    calls.push({ url, options });
    if (url.endsWith("/csrf")) return json({ csrf_token: "csrf-token" });
    if (url.endsWith("/account/auth-methods/password")) {
      unlinkAttempts += 1;
      if (unlinkAttempts === 1) {
        return json(
          {
            error: {
              code: "recent_authentication_required",
              message: "Authenticate again to continue.",
            },
            authentication_challenge: {
              id: "11111111-1111-1111-1111-111111111111",
              expires_at: "2026-09-23T12:05:00Z",
            },
            reauthenticate_url: "/auth/reauthenticate?challenge=opaque",
          },
          428,
        );
      }
      return new Response(null, { status: 204 });
    }
    if (url.endsWith("/reauthenticate/password")) {
      return new Response(null, { status: 204 });
    }
    throw new Error(`Unexpected request: ${url}`);
  };

  let challengeError;
  try {
    await unlinkAuthMethod("password");
  } catch (error) {
    challengeError = error;
  }

  assert.ok(challengeError instanceof APIError);
  assert.equal(isRecentAuthenticationRequired(challengeError), true);
  assert.equal(
    challengeError.authenticationChallenge.id,
    "11111111-1111-1111-1111-111111111111",
  );
  assert.equal(
    challengeError.reauthenticateURL,
    "/auth/reauthenticate?challenge=opaque",
  );

  await reauthenticateWithPassword(
    challengeError.authenticationChallenge.id,
    "password123",
  );
  await unlinkAuthMethod("password");

  const mutations = calls.filter(({ url }) => !url.endsWith("/csrf"));
  assert.deepEqual(
    mutations.map(({ url, options }) => `${options.method} ${url}`),
    [
      "DELETE /auth/api/v1/account/auth-methods/password",
      "POST /auth/api/v1/reauthenticate/password",
      "DELETE /auth/api/v1/account/auth-methods/password",
    ],
  );
  assert.deepEqual(JSON.parse(mutations[1].options.body), {
    authentication_challenge_id:
      "11111111-1111-1111-1111-111111111111",
    password: "password123",
  });
});

test("account management uses only browser API routes", async () => {
  const calls = [];
  const account = {
    user: { id: "user-1", email: "user@example.com", username: "user" },
    sessions: [],
    auth_methods: [{ provider: "password" }],
    passkeys: [],
  };

  globalThis.fetch = async (url, options = {}) => {
    calls.push({ url, options });
    if (url.endsWith("/csrf")) return json({ csrf_token: "csrf-token" });
    if (url.endsWith("/account") && !options.method) return json(account);
    if (url.endsWith("/email-change/challenges")) {
      return json({ challenge_id: "email-challenge" }, 202);
    }
    return new Response(null, { status: 204 });
  };

  assert.deepEqual(await getAccount(), account);
  await changeUsername("new-name");
  await startEmailChange("new@example.com");
  await verifyEmailChange("email-challenge", "123456");
  await addPassword("password123");
  await changePassword("password123", "password456");
  await linkGoogle("google-token", "google-nonce");
  await linkApple("apple-code", "apple-state");
  await unlinkAuthMethod("google");
  await deletePasskey("passkey/1");
  await revokeOtherSessions();
  await revokeSession("session/1");

  const requests = calls.filter(({ url }) => !url.endsWith("/csrf"));
  assert.deepEqual(
    requests.map(({ url, options }) => `${options.method || "GET"} ${url}`),
    [
      "GET /auth/api/v1/account",
      "PATCH /auth/api/v1/account/username",
      "POST /auth/api/v1/account/email-change/challenges",
      "POST /auth/api/v1/account/email-change/challenges/verify",
      "POST /auth/api/v1/account/password",
      "PUT /auth/api/v1/account/password",
      "POST /auth/api/v1/account/auth-methods/google",
      "POST /auth/api/v1/account/auth-methods/apple",
      "DELETE /auth/api/v1/account/auth-methods/google",
      "DELETE /auth/api/v1/account/passkeys/passkey%2F1",
      "DELETE /auth/api/v1/account/sessions/others",
      "DELETE /auth/api/v1/account/sessions/session%2F1",
    ],
  );
  assert.ok(
    requests
      .slice(1)
      .every(({ options }) => options.headers["X-CSRF-Token"] === "csrf-token"),
  );
});

test("external providers and passkeys use the reauthentication challenge", async () => {
  const calls = [];

  globalThis.fetch = async (url, options = {}) => {
    calls.push({ url, options });
    if (url.endsWith("/csrf")) return json({ csrf_token: "csrf-token" });
    if (url.endsWith("/reauthenticate/passkeys/options")) {
      return json({ challenge_id: "webauthn-challenge", options: {} });
    }
    if (url.endsWith("/passkeys/authenticate/options")) {
      return json({ challenge_id: "login-challenge", options: {} });
    }
    if (url.endsWith("/passkeys/register/options")) {
      return json({ challenge_id: "registration-challenge", options: {} });
    }
    return new Response(null, { status: 204 });
  };

  await reauthenticateWithGoogle("auth-challenge", "google-token", "nonce");
  await reauthenticateWithApple("auth-challenge", "apple-code", "apple-state");
  await beginPasskeyAuthentication();
  await finishPasskeyAuthentication("login-challenge", {
    id: "login-credential",
  });
  await beginPasskeyReauthentication("auth-challenge");
  await finishPasskeyReauthentication(
    "auth-challenge",
    "webauthn-challenge",
    { id: "credential" },
  );
  await beginPasskeyRegistration();
  await finishPasskeyRegistration(
    "registration-challenge",
    { id: "credential" },
    "Laptop",
    "macOS",
  );

  const bodies = calls
    .filter(({ url }) => !url.endsWith("/csrf"))
    .map(({ options }) => (options.body ? JSON.parse(options.body) : null));
  assert.deepEqual(bodies, [
    {
      authentication_challenge_id: "auth-challenge",
      credential: "google-token",
      nonce: "nonce",
    },
    {
      authentication_challenge_id: "auth-challenge",
      code: "apple-code",
      state: "apple-state",
    },
    null,
    {
      challenge_id: "login-challenge",
      credential: { id: "login-credential" },
    },
    { authentication_challenge_id: "auth-challenge" },
    {
      authentication_challenge_id: "auth-challenge",
      challenge_id: "webauthn-challenge",
      credential: { id: "credential" },
    },
    null,
    {
      challenge_id: "registration-challenge",
      credential: { id: "credential" },
      name: "Laptop",
      platform_hint: "macOS",
    },
  ]);
});
