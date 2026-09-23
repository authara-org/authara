import { showRedirecting } from "./ui";
import {
  notifyRecentAuthenticationComplete,
  requestRecentAuthentication,
} from "./recentAuthentication";

type GoogleCredentialResponse = { credential?: string };
type GoogleCredentialHandler = (
  response: GoogleCredentialResponse,
) => Promise<void>;

declare global {
  interface Window {
    autharaGoogleCallback?: (
      response: GoogleCredentialResponse,
    ) => void | Promise<void>;
    autharaGoogleCredentialHandler?: GoogleCredentialHandler;
    autharaGoogleCredentialQueue?: GoogleCredentialResponse[];
  }
}

function getGoogleFlowButton(): HTMLElement | null {
  return document.querySelector("[data-google-flow]");
}

function getCSRFToken(): string {
  const match = document.cookie.match(/(?:^|;\s*)authara_csrf=([^;]+)/);
  return match ? decodeURIComponent(match[1]) : "";
}

function getGoogleNonce(): string {
  const onload = document.querySelector<HTMLElement>("#g_id_onload");
  return onload?.dataset.nonce || "";
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

async function handleGoogleCredentialAttempt(
  response: GoogleCredentialResponse,
  allowRecentAuthenticationRetry: boolean,
): Promise<void> {
  const credential = response?.credential;
  if (!credential) return;

  const btn = getGoogleFlowButton();
  const flow = btn?.dataset.googleFlow || "login";
  const returnTo = getReturnTo(btn);
  const provider = btn?.dataset.provider || "google";
  const linkID = btn?.dataset.linkId || "";
  const authenticationChallengeID = getAuthenticationChallengeID(btn);

  const form = new URLSearchParams();
  form.set("credential", credential);
  form.set("flow", flow);
  form.set("nonce", getGoogleNonce());
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

    if (flow === "link") {
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
            return handleGoogleCredentialAttempt(response, false);
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

      form.set("link_id", data.link_id);
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
        return handleGoogleCredentialAttempt(response, false);
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

const handleGoogleCredential: GoogleCredentialHandler = (response) =>
  handleGoogleCredentialAttempt(response, true);

window.autharaGoogleCredentialHandler = handleGoogleCredential;

if (typeof window.autharaGoogleCallback !== "function") {
  window.autharaGoogleCallback = (response: GoogleCredentialResponse) => {
    void handleGoogleCredential(response);
  };
}

for (const response of window.autharaGoogleCredentialQueue?.splice(0) ?? []) {
  void handleGoogleCredential(response);
}
