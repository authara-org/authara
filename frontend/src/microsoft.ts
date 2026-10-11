import type { AuthenticationResult } from "@azure/msal-browser";

import { showRedirecting } from "./ui";
import {
  notifyRecentAuthenticationComplete,
  requestRecentAuthentication,
} from "./recentAuthentication";
import { showToast } from "./toast";

type MicrosoftOptions = {
  client_id: string;
  authority: string;
  nonce: string;
};

type MicrosoftApplication = {
  loginPopup(request: {
    scopes: string[];
    nonce: string;
    prompt: string;
    redirectUri: string;
    overrideInteractionInProgress: boolean;
  }): Promise<AuthenticationResult>;
};

type MicrosoftRuntime = {
  configuration: MicrosoftOptions;
  application: MicrosoftApplication;
};

const initializedButtons = new WeakSet<HTMLButtonElement>();
const microsoftButtons = new Set<HTMLButtonElement>();
let preparedRuntime: MicrosoftRuntime | undefined;
let runtimePromise: Promise<MicrosoftRuntime> | undefined;

function csrfToken(): string {
  const match = document.cookie.match(/(?:^|;\s*)authara_csrf=([^;]+)/);
  return match ? decodeURIComponent(match[1]) : "";
}

function invitationToken(destination: string): string {
  const url = new URL(destination, window.location.origin);
  return url.origin === window.location.origin &&
    url.pathname === "/auth/invitations/accept"
    ? url.searchParams.get("token") || ""
    : "";
}

async function options(): Promise<MicrosoftOptions> {
  const response = await fetch("/auth/api/v1/oauth/microsoft/options", {
    credentials: "same-origin",
  });
  if (!response.ok) throw new Error("Microsoft authentication is unavailable.");
  return response.json() as Promise<MicrosoftOptions>;
}

function setButtonsReady(ready: boolean): void {
  for (const button of microsoftButtons) {
    if (!button.isConnected) {
      microsoftButtons.delete(button);
      continue;
    }
    button.disabled = !ready;
    button.setAttribute("aria-disabled", String(!ready));
  }
}

function prepareMicrosoft(): Promise<MicrosoftRuntime> {
  if (preparedRuntime) return Promise.resolve(preparedRuntime);
  runtimePromise ??= Promise.all([options(), import("@azure/msal-browser")])
    .then(async ([configuration, { PublicClientApplication }]) => {
      const redirectUri = `${window.location.origin}/auth/oauth/microsoft/redirect`;
      const application = new PublicClientApplication({
        auth: {
          clientId: configuration.client_id,
          authority: configuration.authority,
          redirectUri,
        },
        cache: { cacheLocation: "sessionStorage" },
      });
      await application.initialize();
      preparedRuntime = { configuration, application };
      return preparedRuntime;
    })
    .catch((error) => {
      runtimePromise = undefined;
      throw error;
    });
  return runtimePromise;
}

function authorize(runtime: MicrosoftRuntime): Promise<AuthenticationResult> {
  const redirectUri = `${window.location.origin}/auth/oauth/microsoft/redirect`;
  return runtime.application.loginPopup({
    scopes: ["openid", "profile", "email"],
    nonce: runtime.configuration.nonce,
    prompt: "select_account",
    redirectUri,
    overrideInteractionInProgress: true,
  });
}

async function responseError(response: Response): Promise<Error> {
  try {
    const data = (await response.json()) as { error?: { message?: string } };
    return new Error(
      data.error?.message || "Microsoft authentication was not accepted.",
    );
  } catch {
    return new Error("Microsoft authentication was not accepted.");
  }
}

async function submit(
  button: HTMLButtonElement,
  configuration: MicrosoftOptions,
  result: AuthenticationResult,
  allowRecentAuthenticationRetry: boolean,
): Promise<void> {
  if (!result.idToken) throw new Error("Microsoft did not return an ID token.");

  const flow = button.dataset.microsoftFlow || "login";
  const destination = button.dataset.returnTo || "/";
  let endpoint = "/auth/api/v1/oauth/microsoft?audience=app";
  const body: Record<string, string> = {
    credential: result.idToken,
    nonce: configuration.nonce,
  };
  const invite = invitationToken(destination);
  if (invite) body.invitation_token = invite;

  if (flow === "link") {
    endpoint = "/auth/api/v1/account/auth-methods/microsoft";
  } else if (flow === "proof") {
    const linkID = button.dataset.linkId || "";
    endpoint = `/auth/api/v1/provider-links/recovery/${encodeURIComponent(linkID)}/microsoft?audience=app`;
  } else if (flow === "reauthenticate") {
    endpoint = "/auth/api/v1/reauthenticate/microsoft";
    body.authentication_challenge_id =
      button.dataset.authenticationChallengeId || "";
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
      return submit(button, configuration, result, false);
    }
  }
  if (response.status === 409 && flow === "login") {
    const data = (await response.json()) as {
      recovery?: { link_id?: string };
    };
    if (data.recovery?.link_id) {
      const url = new URL(
        "/auth/provider-links/confirm",
        window.location.origin,
      );
      url.searchParams.set("link_id", data.recovery.link_id);
      url.searchParams.set("return_to", destination);
      showRedirecting();
      window.location.href = `${url.pathname}${url.search}`;
      return;
    }
  }
  if (!response.ok) throw await responseError(response);

  if (flow === "reauthenticate" && notifyRecentAuthenticationComplete()) return;
  showRedirecting();
  window.location.href = flow === "link" ? "/auth/account" : destination;
}

async function begin(
  button: HTMLButtonElement,
  allowRecentAuthenticationRetry: boolean,
): Promise<void> {
  if (button.dataset.microsoftBusy === "true") return;
  button.dataset.microsoftBusy = "true";
  button.disabled = true;
  button.setAttribute("aria-busy", "true");
  const runtime = preparedRuntime;
  if (!runtime) {
    delete button.dataset.microsoftBusy;
    button.removeAttribute("aria-busy");
    setButtonsReady(false);
    void prepareMicrosoft().then(() => setButtonsReady(true));
    showToast("error", "Microsoft authentication is still loading.");
    return;
  }
  preparedRuntime = undefined;
  runtimePromise = undefined;
  setButtonsReady(false);
  const authorization = authorize(runtime);
  try {
    const result = await authorization;
    await submit(
      button,
      runtime.configuration,
      result,
      allowRecentAuthenticationRetry,
    );
  } catch (error) {
    const code =
      typeof error === "object" && error !== null && "errorCode" in error
        ? String(error.errorCode)
        : "";
    if (code !== "user_cancelled") {
      const message =
        error instanceof Error
          ? error.message
          : "Microsoft authentication failed.";
      showToast("error", message);
      console.error("Microsoft authentication failed.", error);
    }
  } finally {
    delete button.dataset.microsoftBusy;
    button.removeAttribute("aria-busy");
    void prepareMicrosoft()
      .then(() => setButtonsReady(true))
      .catch((error) => {
        console.error("Microsoft authentication preparation failed.", error);
      });
  }
}

export function initMicrosoftSignin(root: ParentNode): void {
  const buttons = Array.from(
    root.querySelectorAll<HTMLButtonElement>("[data-microsoft-button]"),
  ).filter((button) => !initializedButtons.has(button));
  for (const button of buttons) {
    initializedButtons.add(button);
    microsoftButtons.add(button);
    button.disabled = !preparedRuntime;
    button.setAttribute("aria-disabled", String(!preparedRuntime));
    button.addEventListener("click", (event) => {
      event.preventDefault();
      void begin(button, true);
    });
  }
  if (buttons.length > 0) {
    void prepareMicrosoft()
      .then(() => setButtonsReady(true))
      .catch((error) => {
        showToast("error", "Microsoft authentication is unavailable.");
        console.error("Microsoft authentication preparation failed.", error);
      });
  }
}
