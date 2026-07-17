const API = "/auth/api/v1";

export class APIError extends Error {
  constructor(message, status, code = "") {
    super(message);
    this.name = "APIError";
    this.status = status;
    this.code = code;
  }
}

async function request(path, options = {}) {
  const response = await fetch(path, {
    credentials: "same-origin",
    ...options,
  });

  if (!response.ok) {
    let body;
    try {
      body = await response.json();
    } catch {
      // The status is enough when an upstream does not return Authara's JSON envelope.
    }
    throw new APIError(
      body?.error?.message || `Request failed (${response.status})`,
      response.status,
      body?.error?.code,
    );
  }

  if (
    response.status === 204 ||
    !response.headers.get("content-type")?.includes("application/json")
  ) {
    return null;
  }
  return response.json();
}

async function mutate(path) {
  const { csrf_token: csrfToken } = await request(`${API}/csrf`);
  return request(path, {
    method: "POST",
    headers: { "X-CSRF-Token": csrfToken },
  });
}

let refreshPromise;

export function refreshSession() {
  if (!refreshPromise) {
    refreshPromise = mutate(`${API}/sessions/refresh?audience=app`).finally(
      () => {
        refreshPromise = undefined;
      },
    );
  }
  return refreshPromise;
}

export async function getUserWithRefresh() {
  try {
    return await request(`${API}/user`);
  } catch (error) {
    if (!(error instanceof APIError) || error.status !== 401) {
      throw error;
    }
  }

  await refreshSession();
  return request(`${API}/user`);
}

export async function loadDashboard() {
  const user = await getUserWithRefresh();

  const [organizationList, currentOrganization, memberList] = await Promise.all(
    [
      request(`${API}/organizations`),
      request(`${API}/organizations/current`),
      request(`${API}/organizations/current/members`).catch((error) => {
        if (error instanceof APIError && error.status === 403) {
          return null;
        }
        throw error;
      }),
    ],
  );

  return {
    user,
    organizations: organizationList.organizations,
    currentOrganization,
    members: memberList?.members ?? null,
  };
}

export async function switchOrganization(organizationID) {
  await mutate(
    `${API}/organizations/${encodeURIComponent(organizationID)}/switch?audience=app`,
  );
}

export async function logout() {
  await mutate(`${API}/sessions/logout`);
}
