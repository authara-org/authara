import {
  beginPasskeyAuthentication,
  beginPasskeyReauthentication,
  beginPasskeyRegistration,
  finishPasskeyAuthentication,
  finishPasskeyReauthentication,
  finishPasskeyRegistration,
} from "./api.js";

function supported(operation) {
  if (!window.PublicKeyCredential || !navigator.credentials?.[operation]) {
    throw new Error("Passkeys are not supported by this browser.");
  }
}

function base64urlToBuffer(value) {
  const normalized = value.replace(/-/g, "+").replace(/_/g, "/");
  const padded = normalized.padEnd(
    normalized.length + ((4 - (normalized.length % 4)) % 4),
    "=",
  );
  const binary = window.atob(padded);
  return Uint8Array.from(binary, (character) => character.charCodeAt(0));
}

function bufferToBase64url(buffer) {
  if (!buffer) return "";
  const binary = Array.from(new Uint8Array(buffer), (byte) =>
    String.fromCharCode(byte),
  ).join("");
  return window
    .btoa(binary)
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=+$/g, "");
}

function creationOptions(options) {
  return {
    ...options,
    challenge: base64urlToBuffer(String(options.challenge)),
    user: {
      ...options.user,
      id: base64urlToBuffer(String(options.user.id)),
    },
    excludeCredentials: (options.excludeCredentials || []).map((credential) => ({
      ...credential,
      id: base64urlToBuffer(String(credential.id)),
    })),
  };
}

function requestOptions(options) {
  return {
    ...options,
    challenge: base64urlToBuffer(String(options.challenge)),
    allowCredentials: (options.allowCredentials || []).map((credential) => ({
      ...credential,
      id: base64urlToBuffer(String(credential.id)),
    })),
  };
}

function registrationCredential(credential) {
  const response = credential.response;
  return {
    id: credential.id,
    rawId: bufferToBase64url(credential.rawId),
    type: credential.type,
    authenticatorAttachment: credential.authenticatorAttachment,
    response: {
      clientDataJSON: bufferToBase64url(response.clientDataJSON),
      attestationObject: bufferToBase64url(response.attestationObject),
      transports: response.getTransports?.(),
    },
    clientExtensionResults: credential.getClientExtensionResults(),
  };
}

function authenticationCredential(credential) {
  const response = credential.response;
  return {
    id: credential.id,
    rawId: bufferToBase64url(credential.rawId),
    type: credential.type,
    authenticatorAttachment: credential.authenticatorAttachment,
    response: {
      clientDataJSON: bufferToBase64url(response.clientDataJSON),
      authenticatorData: bufferToBase64url(response.authenticatorData),
      signature: bufferToBase64url(response.signature),
      userHandle: bufferToBase64url(response.userHandle),
    },
    clientExtensionResults: credential.getClientExtensionResults(),
  };
}

function platformHint() {
  return navigator.userAgentData?.platform || navigator.platform || "";
}

export async function registerPasskey(name) {
  supported("create");
  const data = await beginPasskeyRegistration();
  const credential = await navigator.credentials.create({
    publicKey: creationOptions(data.options.publicKey),
  });
  if (!credential) throw new Error("Passkey setup was cancelled.");
  await finishPasskeyRegistration(
    data.challenge_id,
    registrationCredential(credential),
    name,
    platformHint(),
  );
}

export async function authenticateWithPasskey() {
  supported("get");
  const data = await beginPasskeyAuthentication();
  const credential = await navigator.credentials.get({
    publicKey: requestOptions(data.options.publicKey),
    mediation: data.options.mediation,
  });
  if (!credential) throw new Error("Passkey sign-in was cancelled.");
  await finishPasskeyAuthentication(
    data.challenge_id,
    authenticationCredential(credential),
  );
}

export async function reauthenticateWithPasskey(authenticationChallengeID) {
  supported("get");
  const data = await beginPasskeyReauthentication(authenticationChallengeID);
  const credential = await navigator.credentials.get({
    publicKey: requestOptions(data.options.publicKey),
  });
  if (!credential) throw new Error("Passkey authentication was cancelled.");
  await finishPasskeyReauthentication(
    authenticationChallengeID,
    data.challenge_id,
    authenticationCredential(credential),
  );
}
