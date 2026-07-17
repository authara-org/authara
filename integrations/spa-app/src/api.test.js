import assert from "node:assert/strict";
import { afterEach, test } from "node:test";

import { getUserWithRefresh, loadDashboard } from "./api.js";

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

test("dashboard refreshes once before loading protected data", async () => {
  const calls = [];
  let userCalls = 0;

  globalThis.fetch = async (url, options = {}) => {
    calls.push(`${options.method || "GET"} ${url}`);
    if (url.endsWith("/user")) {
      userCalls += 1;
      return userCalls === 1
        ? json(
            { error: { code: "unauthorized", message: "Unauthorized" } },
            401,
          )
        : json({ id: "user-1", email: "user@example.com", roles: [] });
    }
    if (url.endsWith("/csrf")) return json({ csrf_token: "csrf-token" });
    if (url.includes("/sessions/refresh"))
      return new Response(null, { status: 200 });
    if (url.endsWith("/organizations")) return json({ organizations: [] });
    if (url.endsWith("/organizations/current"))
      return json({ id: "org-1", name: "Personal", role: "owner" });
    if (url.endsWith("/organizations/current/members")) {
      return json(
        { error: { code: "forbidden", message: "Unavailable" } },
        403,
      );
    }
    throw new Error(`Unexpected request: ${url}`);
  };

  const dashboard = await loadDashboard();

  assert.equal(dashboard.user.id, "user-1");
  assert.equal(dashboard.members, null);
  assert.deepEqual(calls.slice(0, 4), [
    "GET /auth/api/v1/user",
    "GET /auth/api/v1/csrf",
    "POST /auth/api/v1/sessions/refresh?audience=app",
    "GET /auth/api/v1/user",
  ]);
  assert.ok(calls.indexOf("GET /auth/api/v1/organizations") > 3);
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
