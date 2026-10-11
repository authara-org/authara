import { getMicrosoftOptions } from "./api.js";

let preparedRuntime;
let runtimePromise;

export function prepareMicrosoft() {
  if (preparedRuntime) return Promise.resolve(preparedRuntime);
  runtimePromise ??= Promise.all([
    getMicrosoftOptions(),
    import("@azure/msal-browser"),
  ])
    .then(async ([options, { PublicClientApplication }]) => {
      const redirectUri = `${window.location.origin}/auth/oauth/microsoft/redirect`;
      const application = new PublicClientApplication({
        auth: {
          clientId: options.client_id,
          authority: options.authority,
          redirectUri,
        },
        cache: { cacheLocation: "sessionStorage" },
      });
      await application.initialize();
      preparedRuntime = { application, options, redirectUri };
      return preparedRuntime;
    })
    .catch((error) => {
      runtimePromise = undefined;
      throw error;
    });
  return runtimePromise;
}

export function authorizeWithMicrosoft() {
  const runtime = preparedRuntime;
  if (!runtime) {
    return Promise.reject(
      new Error("Microsoft authentication is still loading."),
    );
  }
  preparedRuntime = undefined;
  runtimePromise = undefined;
  const resultPromise = runtime.application.loginPopup({
    scopes: ["openid", "profile", "email"],
    nonce: runtime.options.nonce,
    prompt: "select_account",
    redirectUri: runtime.redirectUri,
    overrideInteractionInProgress: true,
  });
  return resultPromise.then((result) => {
    if (!result.idToken) {
      throw new Error("Microsoft did not return an ID token.");
    }
    return { credential: result.idToken, nonce: runtime.options.nonce };
  });
}

export function isMicrosoftCancellation(error) {
  const code = String(error?.errorCode || error?.code || "").toLowerCase();
  return (
    code === "user_cancelled" ||
    code === "user_canceled" ||
    code === "popup_window_error" ||
    code === "empty_window_error"
  );
}
