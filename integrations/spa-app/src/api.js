const API = "/auth/api/v1";
const BACKEND = "/spa/api/v1";

export class APIError extends Error {
  constructor(message, status, code = "", response = null) {
    super(message);
    this.name = "APIError";
    this.status = status;
    this.code = code;
    this.response = response;
    this.authenticationChallenge = response?.authentication_challenge ?? null;
    this.reauthenticateURL = response?.reauthenticate_url ?? "";
  }
}

export function isRecentAuthenticationRequired(error) {
  return (
    error instanceof APIError &&
    error.status === 428 &&
    error.code === "recent_authentication_required" &&
    Boolean(error.authenticationChallenge?.id)
  );
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
      body,
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

async function mutateBackend(path, body, method = "POST") {
  try {
    return await mutate(path, body, method);
  } catch (error) {
    if (!(error instanceof APIError) || error.status !== 401) throw error;
  }

  await refreshSession();
  return mutate(path, body, method);
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
  return requestWithRefresh(`${API}/user`);
}

async function requestWithRefresh(path) {
  try {
    return await request(path);
  } catch (error) {
    if (!(error instanceof APIError) || error.status !== 401) {
      throw error;
    }
  }

  await refreshSession();
  return request(path);
}

export function login(identifier, password) {
  return mutate(`${API}/login?audience=app`, { identifier, password });
}

export function getGoogleOptions() {
  return request(`${API}/oauth/google/options`);
}

export function loginWithGoogle(credential, nonce) {
  return mutate(`${API}/oauth/google?audience=app`, { credential, nonce });
}

export function beginPasskeyAuthentication() {
  return mutate(`${API}/passkeys/authenticate/options`);
}

export function finishPasskeyAuthentication(challengeID, credential) {
  return mutate(`${API}/passkeys/authenticate/finish?audience=app`, {
    challenge_id: challengeID,
    credential,
  });
}

function signupBody(email, password, invitationCode) {
  const body = { email, password };
  if (invitationCode.trim()) body.invitation_code = invitationCode.trim();
  return body;
}

export function signupDirect(email, password, invitationCode = "") {
  return mutate(
    `${API}/signup/direct?audience=app`,
    signupBody(email, password, invitationCode),
  );
}

export function startSignupChallenge(email, password, invitationCode = "") {
  return mutate(
    `${API}/signup/challenges?audience=app`,
    signupBody(email, password, invitationCode),
  );
}

export function verifySignupChallenge(challengeID, code) {
  return mutate(`${API}/signup/challenges/verify?audience=app`, {
    challenge_id: challengeID,
    code,
  });
}

export function resendSignupChallenge(challengeID) {
  return mutate(`${API}/challenges/resend`, {
    challenge_id: challengeID,
  });
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
  return mutateBackend(`${BACKEND}/organizations`, { name });
}

export function updateOrganization(organizationID, name) {
  return mutate(
    `${API}/organizations/${encodeURIComponent(organizationID)}`,
    { name },
    "PATCH",
  );
}

export function inviteMember(organizationID, email, role = "member") {
  return mutateBackend(
    `${BACKEND}/organizations/${encodeURIComponent(organizationID)}/invitations`,
    { email, role },
  );
}

export function revokeInvitation(organizationID, invitationID) {
  return mutate(
    `${API}/organizations/${encodeURIComponent(organizationID)}/invitations/${encodeURIComponent(invitationID)}/revoke`,
  );
}

export function resendInvitation(organizationID, invitationID) {
  return mutateBackend(
    `${BACKEND}/organizations/${encodeURIComponent(organizationID)}/invitations/${encodeURIComponent(invitationID)}/resend`,
  );
}

export function deleteOrganization(organizationID) {
  return mutateBackend(
    `${BACKEND}/organizations/${encodeURIComponent(organizationID)}`,
    undefined,
    "DELETE",
  );
}

export function removeOrganizationMember(organizationID, userID) {
  return mutateBackend(
    `${BACKEND}/organizations/${encodeURIComponent(organizationID)}/members/${encodeURIComponent(userID)}`,
    undefined,
    "DELETE",
  );
}

export function transferOrganizationOwnership(organizationID, newOwnerUserID) {
  return mutateBackend(
    `${BACKEND}/organizations/${encodeURIComponent(organizationID)}/ownership-transfer`,
    { new_owner_user_id: newOwnerUserID },
  );
}

export function deleteCurrentAccount() {
  return mutateBackend(`${BACKEND}/account`, undefined, "DELETE");
}

export async function switchOrganization(organizationID) {
  await mutate(
    `${API}/organizations/${encodeURIComponent(organizationID)}/switch?audience=app`,
  );
}

export async function logout() {
  await mutate(`${API}/sessions/logout`);
}

export function getAccount() {
  return requestWithRefresh(`${API}/account`);
}

export function changeUsername(username) {
  return mutate(`${API}/account/username`, { username }, "PATCH");
}

export function startEmailChange(newEmail) {
  return mutate(`${API}/account/email-change/challenges`, {
    new_email: newEmail,
  });
}

export function verifyEmailChange(challengeID, code) {
  return mutate(`${API}/account/email-change/challenges/verify`, {
    challenge_id: challengeID,
    code,
  });
}

export function addPassword(password) {
  return mutate(`${API}/account/password`, { password });
}

export function changePassword(currentPassword, newPassword) {
  return mutate(
    `${API}/account/password`,
    { current_password: currentPassword, new_password: newPassword },
    "PUT",
  );
}

export function linkGoogle(credential, nonce) {
  return mutate(`${API}/account/auth-methods/google`, { credential, nonce });
}

export function unlinkAuthMethod(provider) {
  return mutate(
    `${API}/account/auth-methods/${encodeURIComponent(provider)}`,
    undefined,
    "DELETE",
  );
}

export function deletePasskey(passkeyID) {
  return mutate(
    `${API}/account/passkeys/${encodeURIComponent(passkeyID)}`,
    undefined,
    "DELETE",
  );
}

export function revokeOtherSessions() {
  return mutate(`${API}/account/sessions/others`, undefined, "DELETE");
}

export function revokeSession(sessionID) {
  return mutate(
    `${API}/account/sessions/${encodeURIComponent(sessionID)}`,
    undefined,
    "DELETE",
  );
}

export function beginPasskeyRegistration() {
  return mutate(`${API}/passkeys/register/options`);
}

export function finishPasskeyRegistration(challengeID, credential, name, hint) {
  return mutate(`${API}/passkeys/register/finish`, {
    challenge_id: challengeID,
    credential,
    name,
    platform_hint: hint,
  });
}

export function reauthenticateWithPassword(authenticationChallengeID, password) {
  return mutate(`${API}/reauthenticate/password`, {
    authentication_challenge_id: authenticationChallengeID,
    password,
  });
}

export function reauthenticateWithGoogle(
  authenticationChallengeID,
  credential,
  nonce,
) {
  return mutate(`${API}/reauthenticate/google`, {
    authentication_challenge_id: authenticationChallengeID,
    credential,
    nonce,
  });
}

export function beginPasskeyReauthentication(authenticationChallengeID) {
  return mutate(`${API}/reauthenticate/passkeys/options`, {
    authentication_challenge_id: authenticationChallengeID,
  });
}

export function finishPasskeyReauthentication(
  authenticationChallengeID,
  challengeID,
  credential,
) {
  return mutate(`${API}/reauthenticate/passkeys/finish`, {
    authentication_challenge_id: authenticationChallengeID,
    challenge_id: challengeID,
    credential,
  });
}
