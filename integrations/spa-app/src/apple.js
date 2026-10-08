import { getAppleOptions } from "./api.js";

const appleIdentityURL =
  "https://appleid.cdn-apple.com/appleauth/static/jsapi/appleid/1/en_US/appleid.auth.js";

let appleIdentityPromise;
let renderOptionsPromise;

export function loadAppleIdentity() {
  if (window.AppleID?.auth) return Promise.resolve(window.AppleID.auth);
  if (appleIdentityPromise) return appleIdentityPromise;

  appleIdentityPromise = new Promise((resolve, reject) => {
    const script = document.createElement("script");
    script.src = appleIdentityURL;
    script.async = true;
    script.onload = () =>
      window.AppleID?.auth
        ? resolve(window.AppleID.auth)
        : reject(new Error("Apple authentication did not initialize."));
    script.onerror = () =>
      reject(new Error("Could not load Apple authentication."));
    document.head.append(script);
  });

  return appleIdentityPromise;
}

function config(options) {
  return {
    clientId: options.client_id,
    scope: "email",
    redirectURI: options.redirect_uri,
    state: options.state,
    nonce: options.nonce,
    usePopup: true,
  };
}

function getRenderOptions() {
  if (!renderOptionsPromise) {
    renderOptionsPromise = getAppleOptions().catch((error) => {
      renderOptionsPromise = undefined;
      throw error;
    });
  }
  return renderOptionsPromise;
}

export function appleErrorMessage(error) {
  const code = typeof error?.error === "string" ? error.error.trim() : "";
  if (typeof error?.message === "string" && error.message.trim()) {
    return error.message.trim();
  }
  return code
    ? `Apple authentication failed (${code}).`
    : "Apple authentication failed.";
}

export function isAppleCancellation(error) {
  const code = typeof error?.error === "string" ? error.error.trim() : "";
  return code === "user_cancelled_authorize" || code === "popup_closed_by_user";
}

export async function prepareAppleButton(element) {
  const [auth, options] = await Promise.all([
    loadAppleIdentity(),
    getRenderOptions(),
  ]);
  auth.init(config(options));
  document
    .querySelectorAll("#appleid-signin")
    .forEach((button) => button.removeAttribute("id"));
  element.id = "appleid-signin";
  auth.renderButton();
  element.removeAttribute("id");
}

export async function authorizeWithApple() {
  const [auth, options] = await Promise.all([
    loadAppleIdentity(),
    getAppleOptions(),
  ]);
  const result = await auth.signIn(config(options));
  const { code, state } = result?.authorization || {};
  if (!code || !state) {
    throw new Error("Apple did not return an authorization code.");
  }
  return { code, state };
}
