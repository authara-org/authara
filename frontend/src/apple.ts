import { showRedirecting } from "./ui";
import {
  notifyRecentAuthenticationComplete,
  requestRecentAuthentication,
} from "./recentAuthentication";
import { showToast } from "./toast";

type AppleOptions = {
  client_id: string;
  redirect_uri: string;
  state: string;
  nonce: string;
};

type AppleAuthorization = { code?: string; state?: string };
type AppleSignInResponse = { authorization?: AppleAuthorization };
type AppleSignInError = { error?: unknown; message?: unknown };

type AppleAuth = {
  init: (options: {
    clientId: string;
    scope: string;
    redirectURI: string;
    state: string;
    nonce: string;
    usePopup: boolean;
  }) => void;
  signIn: (options?: {
    clientId: string;
    scope: string;
    redirectURI: string;
    state: string;
    nonce: string;
    usePopup: boolean;
  }) => Promise<AppleSignInResponse>;
};

declare global {
  interface Window {
    AppleID?: { auth: AppleAuth };
  }
}

const sdkURL =
  "https://appleid.cdn-apple.com/appleauth/static/jsapi/appleid/1/en_US/appleid.auth.js";
let sdkPromise: Promise<AppleAuth> | null = null;
let renderOptionsPromise: Promise<AppleOptions> | null = null;

function loadAppleIdentity(): Promise<AppleAuth> {
  if (window.AppleID?.auth) return Promise.resolve(window.AppleID.auth);
  if (sdkPromise) return sdkPromise;
  sdkPromise = new Promise((resolve, reject) => {
    const existing = document.querySelector<HTMLScriptElement>(
      `script[src="${sdkURL}"]`,
    );
    const script = existing || document.createElement("script");
    const loaded = () =>
      window.AppleID?.auth
        ? resolve(window.AppleID.auth)
        : reject(new Error("Apple Sign In did not initialize."));
    script.addEventListener("load", loaded, { once: true });
    script.addEventListener(
      "error",
      () => reject(new Error("Could not load Apple Sign In.")),
      { once: true },
    );
    if (!existing) {
      script.src = sdkURL;
      script.async = true;
      document.head.appendChild(script);
    }
  });
  return sdkPromise;
}

async function getAppleOptions(): Promise<AppleOptions> {
  const response = await fetch("/auth/api/v1/oauth/apple/options", {
    credentials: "same-origin",
  });
  if (!response.ok) throw new Error("Apple authentication is unavailable.");
  return response.json() as Promise<AppleOptions>;
}

function getRenderOptions(): Promise<AppleOptions> {
  if (!renderOptionsPromise) {
    renderOptionsPromise = getAppleOptions().catch((error) => {
      renderOptionsPromise = null;
      throw error;
    });
  }
  return renderOptionsPromise;
}

function appleConfig(options: AppleOptions) {
  return {
    clientId: options.client_id,
    scope: "email",
    redirectURI: options.redirect_uri,
    state: options.state,
    nonce: options.nonce,
    usePopup: true,
  };
}

function csrfToken(): string {
  const match = document.cookie.match(/(?:^|;\s*)authara_csrf=([^;]+)/);
  return match ? decodeURIComponent(match[1]) : "";
}

function returnTo(button: HTMLElement): string {
  return button.dataset.returnTo || "/";
}

function invitationToken(destination: string): string {
  const url = new URL(destination, window.location.origin);
  return url.origin === window.location.origin &&
    url.pathname === "/auth/invitations/accept"
    ? url.searchParams.get("token") || ""
    : "";
}

function appleErrorDetails(error: unknown): {
  cancelled: boolean;
  message: string;
} {
  const sdkError =
    typeof error === "object" && error !== null
      ? (error as AppleSignInError)
      : undefined;
  const code = typeof sdkError?.error === "string" ? sdkError.error.trim() : "";
  const cancelled =
    code === "user_cancelled_authorize" || code === "popup_closed_by_user";
  if (cancelled) {
    return { cancelled: true, message: "" };
  }
  if (error instanceof Error && error.message) {
    return { cancelled: false, message: error.message };
  }
  if (typeof sdkError?.message === "string" && sdkError.message.trim()) {
    return { cancelled: false, message: sdkError.message.trim() };
  }
  if (code) {
    return {
      cancelled: false,
      message: `Apple authentication failed (${code}).`,
    };
  }
  return {
    cancelled: false,
    message: "Apple authentication failed. Please try again.",
  };
}

async function responseError(
  response: Response,
  fallback: string,
): Promise<Error> {
  try {
    const data = (await response.json()) as {
      error?: { message?: string };
    };
    return new Error(data.error?.message || fallback);
  } catch {
    return new Error(fallback);
  }
}

async function submitAuthorization(
  button: HTMLElement,
  authorization: AppleAuthorization,
  allowRecentAuthenticationRetry = true,
): Promise<void> {
  if (!authorization.code || !authorization.state) {
    throw new Error("Apple did not return an authorization code.");
  }
  const flow = button.dataset.appleFlow || "login";
  const destination = returnTo(button);
  let endpoint = "/auth/api/v1/oauth/apple?audience=app";
  let body: Record<string, string> = {
    code: authorization.code,
    state: authorization.state,
  };
  const token = invitationToken(destination);
  if (token) body.invitation_token = token;
  if (flow === "link") {
    endpoint = "/auth/api/v1/account/auth-methods/apple";
  } else if (flow === "proof") {
    endpoint = `/auth/oauth/apple/proof?return_to=${encodeURIComponent(destination)}`;
    body = {
      ...body,
      link_id: button.dataset.linkId || "",
    };
  } else if (flow === "reauthenticate") {
    endpoint = "/auth/api/v1/reauthenticate/apple";
    body = {
      ...body,
      authentication_challenge_id:
        button.dataset.authenticationChallengeId || "",
    };
  }

  const response = await fetch(endpoint, {
    method: "POST",
    credentials: "same-origin",
    headers: {
      "Content-Type": "application/json",
      "X-CSRF-Token": csrfToken(),
    },
    body: JSON.stringify(body),
  });

  if (response.status === 428 && flow === "link") {
    const data = (await response.json()) as { reauthenticate_url?: string };
    if (allowRecentAuthenticationRetry && data.reauthenticate_url) {
      await requestRecentAuthentication(data.reauthenticate_url);
      return authorizeAndSubmit(button, false);
    }
  }
  if (response.status === 409 && flow === "login") {
    const recoveryLocation = response.headers.get("X-Authara-Redirect");
    if (recoveryLocation) {
      const recoveryURL = new URL(recoveryLocation, window.location.origin);
      recoveryURL.searchParams.set("return_to", destination);
      showRedirecting();
      window.location.href = `${recoveryURL.pathname}${recoveryURL.search}${recoveryURL.hash}`;
      return;
    }
  }
  if (!response.ok) {
    throw await responseError(
      response,
      "Apple authentication was not accepted.",
    );
  }

  if (flow === "reauthenticate" && notifyRecentAuthenticationComplete()) {
    return;
  }
  showRedirecting();
  window.location.href =
    response.headers.get("X-Authara-Redirect") ||
    (flow === "link" ? "/auth/account" : destination);
}

async function authorizeAndSubmit(
  button: HTMLElement,
  allowRecentAuthenticationRetry: boolean,
): Promise<void> {
  const [auth, options] = await Promise.all([
    loadAppleIdentity(),
    getAppleOptions(),
  ]);
  const result = await auth.signIn(appleConfig(options));
  await submitAuthorization(
    button,
    result.authorization || {},
    allowRecentAuthenticationRetry,
  );
}

async function beginAppleSignin(button: HTMLElement): Promise<void> {
  if (button.dataset.appleBusy === "true") return;
  button.dataset.appleBusy = "true";
  const trigger = button.querySelector<HTMLButtonElement>(
    "[data-apple-button]",
  );
  if (trigger) {
    trigger.disabled = true;
    trigger.setAttribute("aria-disabled", "true");
    trigger.setAttribute("aria-busy", "true");
    trigger.classList.add("opacity-70");
  }
  try {
    await authorizeAndSubmit(button, true);
  } catch (error) {
    const details = appleErrorDetails(error);
    if (!details.cancelled) {
      showToast("error", details.message);
      console.error("Apple authentication failed.", error);
    }
  } finally {
    delete button.dataset.appleBusy;
    if (trigger) {
      trigger.disabled = false;
      trigger.setAttribute("aria-disabled", "false");
      trigger.removeAttribute("aria-busy");
      trigger.classList.remove("opacity-70");
    }
  }
}

export async function initAppleSignin(root: ParentNode): Promise<void> {
  const wrappers = Array.from(
    root.querySelectorAll<HTMLElement>(
      ".authara-apple-signin:not([data-apple-initialized])",
    ),
  );
  if (wrappers.length === 0) return;
  try {
    const [auth, options] = await Promise.all([
      loadAppleIdentity(),
      getRenderOptions(),
    ]);
    auth.init(appleConfig(options));
    for (const wrapper of wrappers) {
      const trigger = wrapper.querySelector<HTMLButtonElement>(
        "[data-apple-button]",
      );
      if (!trigger) continue;
      wrapper.dataset.appleInitialized = "true";
      trigger.disabled = false;
      trigger.setAttribute("aria-disabled", "false");
      trigger.addEventListener("click", (event) => {
        event.preventDefault();
        void beginAppleSignin(wrapper);
      });
    }
  } catch (error) {
    console.error("Could not initialize Apple authentication.", error);
  }
}
