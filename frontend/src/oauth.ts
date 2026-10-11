import { showRedirecting } from "./ui";
import {
  notifyRecentAuthenticationComplete,
  requestRecentAuthentication,
} from "./recentAuthentication";

type GoogleCodeResponse = { code?: string; error?: string };
type GoogleCodeClient = { requestCode(): void };
type GoogleIdentity = {
  accounts: {
    oauth2: {
      initCodeClient(options: {
        client_id: string;
        scope: string;
        ux_mode: "popup";
        select_account: boolean;
        callback: (response: GoogleCodeResponse) => void;
        error_callback: () => void;
      }): GoogleCodeClient;
    };
  };
};

declare global {
  interface Window {
    google?: GoogleIdentity;
  }
}

const googleIdentityURL = "https://accounts.google.com/gsi/client";
const initializedGoogleButtons = new WeakSet<HTMLButtonElement>();
let googleIdentityPromise: Promise<GoogleIdentity> | undefined;

function getCSRFToken(): string {
  const match = document.cookie.match(/(?:^|;\s*)authara_csrf=([^;]+)/);
  return match ? decodeURIComponent(match[1]) : "";
}

function getReturnTo(btn: HTMLElement | null): string {
  const fromButton = btn?.dataset.returnTo || "";
  if (fromButton && fromButton !== "/") return fromButton;

  const current = `${window.location.pathname}${window.location.search}`;
  if (
    (window.location.pathname === "/auth/invitations/login" ||
      window.location.pathname === "/auth/invitations/signup") &&
    new URLSearchParams(window.location.search).has("token")
  ) {
    return current;
  }

  return fromButton || "/";
}

function getAuthenticationChallengeID(btn: HTMLElement | null): string {
  return (
    btn?.dataset.authenticationChallengeId ||
    btn?.closest<HTMLElement>("[data-authentication-challenge-id]")?.dataset
      .authenticationChallengeId ||
    ""
  );
}

function authenticationFailureURL(
  flow: string,
  returnTo: string,
  authenticationChallengeID = "",
): string {
  if (flow === "link") return "/auth/account";
  if (flow === "reauthenticate") {
    const params = new URLSearchParams({ return_to: returnTo });
    if (authenticationChallengeID) {
      params.set("authentication_challenge_id", authenticationChallengeID);
    }
    return `/auth/reauthenticate?${params.toString()}`;
  }
  return `/auth/login?return_to=${encodeURIComponent(returnTo)}`;
}

async function handleGoogleAuthorizationAttempt(
  response: GoogleCodeResponse,
  btn: HTMLButtonElement,
  allowRecentAuthenticationRetry: boolean,
  retry: () => void,
): Promise<void> {
  const code = response?.code;
  if (!code) return;

  const flow = btn.dataset.googleFlow || "login";
  const returnTo = getReturnTo(btn);
  const provider = btn?.dataset.provider || "google";
  let linkID = btn?.dataset.linkId || "";
  const authenticationChallengeID = getAuthenticationChallengeID(btn);

  const form = new URLSearchParams();
  form.set("code", code);
  form.set("redirect_uri", window.location.origin);
  form.set("flow", flow);
  if (flow === "reauthenticate") {
    form.set("authentication_challenge_id", authenticationChallengeID);
  }

  try {
    if (flow === "proof") {
      if (!linkID) {
        window.location.href = `/auth/login?return_to=${encodeURIComponent(returnTo)}`;
        return;
      }
      form.set("link_id", linkID);
    }

    if (flow === "link" && !linkID) {
      const startRes = await fetch(
        `/auth/providers/${encodeURIComponent(provider)}/link/start`,
        {
          method: "POST",
          credentials: "include",
          headers: {
            "Content-Type": "application/x-www-form-urlencoded",
            "X-CSRF-Token": getCSRFToken(),
          },
          body: "",
        },
      );

      if (!startRes.ok) {
        if (startRes.status === 428) {
          const data = (await startRes.json()) as {
            reauthenticate_url?: string;
          };
          if (allowRecentAuthenticationRetry && data.reauthenticate_url) {
            await requestRecentAuthentication(data.reauthenticate_url);
            retry();
            return;
          }
        }
        window.location.href = "/auth/account";
        return;
      }

      const data = (await startRes.json()) as { link_id?: string };
      if (!data.link_id) {
        window.location.href = "/auth/account";
        return;
      }

      linkID = data.link_id;
      btn.dataset.linkId = linkID;
    }

    if (flow === "link") {
      form.set("link_id", linkID);
    }

    const res = await fetch(
      `/auth/oauth/google/callback?return_to=${encodeURIComponent(returnTo)}`,
      {
        method: "POST",
        credentials: "include",
        headers: {
          "Content-Type": "application/x-www-form-urlencoded",
          "X-CSRF-Token": getCSRFToken(),
        },
        body: form.toString(),
        redirect: "manual",
      },
    );

    if (res.status >= 300 && res.status < 400) {
      const location = res.headers.get("Location");
      if (location) {
        showRedirecting();
        window.location.href = location;
        return;
      }
    }

    if (res.status === 428) {
      const data = (await res.json()) as { reauthenticate_url?: string };
      if (allowRecentAuthenticationRetry && data.reauthenticate_url) {
        await requestRecentAuthentication(data.reauthenticate_url);
        retry();
        return;
      }
      window.location.href = authenticationFailureURL(
        flow,
        returnTo,
        authenticationChallengeID,
      );
      return;
    }

    if (res.ok) {
      if (flow === "reauthenticate" && notifyRecentAuthenticationComplete()) {
        return;
      }
      const autharaRedirect = res.headers.get("X-Authara-Redirect");
      window.location.href =
        autharaRedirect || (flow === "link" ? "/auth/account" : returnTo);
      return;
    }

    window.location.href = authenticationFailureURL(
      flow,
      returnTo,
      authenticationChallengeID,
    );
  } catch {
    window.location.href = authenticationFailureURL(
      flow,
      returnTo,
      authenticationChallengeID,
    );
  }
}

function loadGoogleIdentity(): Promise<GoogleIdentity> {
  if (window.google?.accounts?.oauth2) return Promise.resolve(window.google);
  if (googleIdentityPromise) return googleIdentityPromise;

  const loading = new Promise<GoogleIdentity>((resolve, reject) => {
    const existing = document.querySelector<HTMLScriptElement>(
      `script[src="${googleIdentityURL}"]`,
    );
    const script = existing ?? document.createElement("script");
    script.addEventListener(
      "load",
      () =>
        window.google?.accounts?.oauth2
          ? resolve(window.google)
          : reject(new Error("Could not load Google Identity Services.")),
      { once: true },
    );
    script.addEventListener(
      "error",
      () => reject(new Error("Could not load Google Identity Services.")),
      { once: true },
    );
    if (!existing) {
      script.src = googleIdentityURL;
      script.async = true;
      document.head.appendChild(script);
    }
  });
  googleIdentityPromise = loading.catch((error) => {
    googleIdentityPromise = undefined;
    throw error;
  });
  return googleIdentityPromise;
}

export async function initGoogleSignin(root: ParentNode): Promise<void> {
  const buttons = Array.from(
    root.querySelectorAll<HTMLButtonElement>("[data-google-button]"),
  ).filter((button) => !initializedGoogleButtons.has(button));
  if (buttons.length === 0) return;

  try {
    const google = await loadGoogleIdentity();
    for (const button of buttons) {
      const clientID = button.dataset.googleClientId;
      if (!clientID) continue;
      let client: GoogleCodeClient;
      client = google.accounts.oauth2.initCodeClient({
        client_id: clientID,
        scope: "openid email profile",
        ux_mode: "popup",
        select_account: true,
        callback: (response) => {
          button.disabled = true;
          void handleGoogleAuthorizationAttempt(response, button, true, () =>
            client.requestCode(),
          ).finally(() => {
            button.disabled = false;
          });
        },
        error_callback: () => {
          button.disabled = false;
        },
      });
      button.addEventListener("click", () => client.requestCode());
      initializedGoogleButtons.add(button);
      button.disabled = false;
    }
  } catch (error) {
    console.error("Google authentication is unavailable.", error);
  }
}
