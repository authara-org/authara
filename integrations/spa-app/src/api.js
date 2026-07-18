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

async function mutate(path, body, method = "POST") {
  const { csrf_token: csrfToken } = await request(`${API}/csrf`);
  const headers = { "X-CSRF-Token": csrfToken };
  if (body !== undefined) headers["Content-Type"] = "application/json";

  return request(path, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
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
  const capabilities = await request(`${API}/capabilities`);

  if (!capabilities.allows_public_organization_management) {
    throw new APIError(
      "Public organization management is disabled in Authara.",
      404,
      "public_organization_management_disabled",
    );
  }

  const organizationID = user.organization?.id;
  if (!organizationID) {
    throw new APIError("The session has no current organization.", 401);
  }

  const encodedOrganizationID = encodeURIComponent(organizationID);
  const allowForbidden = (promise) =>
    promise.catch((error) => {
      if (error instanceof APIError && error.status === 403) return null;
      throw error;
    });

  const [
    organizationResult,
    memberList,
    currentMemberResult,
    invitationList,
    membershipList,
  ] = await Promise.all([
    request(`${API}/organizations/${encodedOrganizationID}`),
    allowForbidden(
      request(`${API}/organizations/${encodedOrganizationID}/members`),
    ),
    allowForbidden(
      request(
        `${API}/organizations/${encodedOrganizationID}/members/${encodeURIComponent(user.id)}`,
      ),
    ),
    allowForbidden(
      request(`${API}/organizations/${encodedOrganizationID}/invitations`),
    ),
    request(`${API}/users/${encodeURIComponent(user.id)}/memberships`),
  ]);

  const invitationDetails = invitationList
    ? await Promise.all(
        invitationList.invitations.map(async (invitation) => {
          const result = await request(
            `${API}/organizations/${encodedOrganizationID}/invitations/${encodeURIComponent(invitation.id)}`,
          );
          return result.invitation;
        }),
      )
    : null;

  return {
    user,
    organizations: membershipList.memberships.map(
      ({ organization, membership }) => ({
        ...organization,
        role: membership.role,
      }),
    ),
    currentOrganization: {
      ...organizationResult.organization,
      role: user.organization.role,
    },
    members: memberList?.members ?? null,
    currentMember: currentMemberResult?.member ?? null,
    invitations: invitationDetails,
    capabilities,
  };
}

export function createOrganization(name) {
  return mutate(`${API}/organizations`, { name });
}

export function updateOrganization(organizationID, name) {
  return mutate(
    `${API}/organizations/${encodeURIComponent(organizationID)}`,
    { name },
    "PATCH",
  );
}

export function inviteMember(organizationID, email) {
  return mutate(
    `${API}/organizations/${encodeURIComponent(organizationID)}/invitations`,
    { email },
  );
}

export function revokeInvitation(organizationID, invitationID) {
  return mutate(
    `${API}/organizations/${encodeURIComponent(organizationID)}/invitations/${encodeURIComponent(invitationID)}/revoke`,
  );
}

export function resendInvitation(organizationID, invitationID) {
  return mutate(
    `${API}/organizations/${encodeURIComponent(organizationID)}/invitations/${encodeURIComponent(invitationID)}/resend`,
  );
}

export async function switchOrganization(organizationID) {
  await mutate(
    `${API}/organizations/${encodeURIComponent(organizationID)}/switch?audience=app`,
  );
}

export async function logout() {
  await mutate(`${API}/sessions/logout`);
}
