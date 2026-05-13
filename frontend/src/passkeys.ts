type PasskeyOptionsResponse = {
  challenge_id: string;
  options: {
    publicKey: PublicKeyCredentialCreationOptions | PublicKeyCredentialRequestOptions;
    mediation?: CredentialMediationRequirement;
  };
};

type PasskeyFinishResponse = {
  ok?: boolean;
  return_to?: string;
};

type PasskeyRegistrationFinishResponse = PasskeyFinishResponse & {
  linkedProvidersHTML?: string;
};

type ToastKind = "success" | "info" | "error";

function getCSRFToken(): string {
  const match = document.cookie.match(/(?:^|;\s*)authara_csrf=([^;]+)/);
  return match ? decodeURIComponent(match[1]) : "";
}

function platformHint(): string {
  const uaData = (navigator as Navigator & { userAgentData?: { platform?: string } }).userAgentData;
  return uaData?.platform || navigator.platform || "";
}

function base64urlToBuffer(value: string): Uint8Array {
  const normalized = value.replace(/-/g, "+").replace(/_/g, "/");
  const padded = normalized.padEnd(normalized.length + ((4 - (normalized.length % 4)) % 4), "=");
  const binary = window.atob(padded);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i += 1) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes;
}

function bufferToBase64url(buffer?: ArrayBuffer | null): string {
  if (!buffer) return "";
  const bytes = new Uint8Array(buffer);
  let binary = "";
  for (const byte of bytes) {
    binary += String.fromCharCode(byte);
  }
  return window
    .btoa(binary)
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=+$/g, "");
}

function normalizeCreationOptions(
  options: PublicKeyCredentialCreationOptions,
): PublicKeyCredentialCreationOptions {
  const publicKey = { ...options } as any;
  publicKey.challenge = base64urlToBuffer(String(options.challenge));
  publicKey.user = { ...options.user, id: base64urlToBuffer(String(options.user.id)) };
  publicKey.excludeCredentials = (options.excludeCredentials || []).map((credential) => ({
    ...credential,
    id: base64urlToBuffer(String(credential.id)),
  }));
  return publicKey;
}

function normalizeRequestOptions(
  options: PublicKeyCredentialRequestOptions,
): PublicKeyCredentialRequestOptions {
  const publicKey = { ...options } as any;
  publicKey.challenge = base64urlToBuffer(String(options.challenge));
  publicKey.allowCredentials = (options.allowCredentials || []).map((credential) => ({
    ...credential,
    id: base64urlToBuffer(String(credential.id)),
  }));
  return publicKey;
}

function serializeRegistrationCredential(credential: PublicKeyCredential) {
  const response = credential.response as AuthenticatorAttestationResponse;
  const transports =
    typeof response.getTransports === "function" ? response.getTransports() : undefined;

  return {
    id: credential.id,
    rawId: bufferToBase64url(credential.rawId),
    type: credential.type,
    authenticatorAttachment: credential.authenticatorAttachment,
    response: {
      clientDataJSON: bufferToBase64url(response.clientDataJSON),
      attestationObject: bufferToBase64url(response.attestationObject),
      transports,
    },
    clientExtensionResults: credential.getClientExtensionResults(),
  };
}

function serializeAuthenticationCredential(credential: PublicKeyCredential) {
  const response = credential.response as AuthenticatorAssertionResponse;

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

async function responseErrorMessage(res: Response, fallback: string): Promise<string> {
  if (!res.ok) {
    try {
      const data = (await res.json()) as { error?: { message?: string } };
      return data.error?.message || fallback;
    } catch {
      return fallback;
    }
  }

  return fallback;
}

async function postJSON<T>(url: string, body?: unknown): Promise<T> {
  const res = await fetch(url, {
    method: "POST",
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      "X-CSRF-Token": getCSRFToken(),
    },
    body: JSON.stringify(body ?? {}),
  });

  if (!res.ok) {
    throw new Error(await responseErrorMessage(res, "Passkey request failed."));
  }

  return (await res.json()) as T;
}

async function postRegistrationFinish(
  body: unknown,
  responseMode: "json" | "linked-providers-section",
): Promise<PasskeyRegistrationFinishResponse> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    "X-CSRF-Token": getCSRFToken(),
  };
  if (responseMode === "linked-providers-section") {
    headers.Accept = "text/html";
    headers["X-Authara-Response"] = "linked-providers-section";
  }

  const res = await fetch("/auth/passkeys/register/finish", {
    method: "POST",
    credentials: "include",
    headers,
    body: JSON.stringify(body ?? {}),
  });

  if (!res.ok) {
    throw new Error(await responseErrorMessage(res, "Could not add passkey."));
  }

  if (responseMode === "linked-providers-section") {
    return { ok: true, linkedProvidersHTML: await res.text() };
  }

  return (await res.json()) as PasskeyFinishResponse;
}

function removeToast(toast: HTMLElement): void {
  if (toast.dataset.removing === "true") return;
  toast.dataset.removing = "true";
  toast.classList.remove("toast-enter");
  toast.classList.add("toast-exit");
  toast.addEventListener("animationend", () => toast.remove(), { once: true });
}

function showToast(kind: ToastKind, message: string): void {
  const container = document.querySelector<HTMLElement>("#toast-container");
  const template = document.querySelector<HTMLTemplateElement>(`#toast-template-${kind}`);
  const templateRoot = template?.content.firstElementChild;

  if (!container || !(templateRoot instanceof HTMLElement)) {
    // The root layout should always provide the toast templates. Keep a visible fallback for partial pages.
    window.alert(message);
    return;
  }

  const toast = templateRoot.cloneNode(true) as HTMLElement;
  const messageEl = toast.querySelector<HTMLElement>("[data-toast-message]");
  if (messageEl) {
    messageEl.textContent = message;
  }

  toast
    .querySelector<HTMLButtonElement>("[data-toast-close]")
    ?.addEventListener("click", () => removeToast(toast));

  window.setTimeout(() => removeToast(toast), 5000);
  container.prepend(toast);
}

function showPasskeyError(message: string): void {
  showToast("error", message);
}

function replaceLinkedProvidersSection(html: string): boolean {
  const current = document.querySelector<HTMLElement>("#linked-providers-section");
  if (!current) return false;

  const template = document.createElement("template");
  template.innerHTML = html.trim();
  const next = template.content.querySelector<HTMLElement>("#linked-providers-section");
  if (!next) return false;

  current.replaceWith(next);

  const htmx = (window as Window & { htmx?: { process?: (element: Element) => void } }).htmx;
  htmx?.process?.(next);
  initPasskeys(next);

  return true;
}

function disableDuplicateButton(button: HTMLButtonElement): void {
  button.disabled = true;
  button.classList.add("opacity-60");
}

function passkeyErrorMessage(
  err: unknown,
  fallback: string,
  cancelled: string,
): string {
  if (err instanceof DOMException) {
    switch (err.name) {
      case "NotAllowedError":
      case "AbortError":
        return cancelled;
      case "SecurityError":
        return "Passkeys require HTTPS or localhost.";
      default:
        return fallback;
    }
  }

  return err instanceof Error ? err.message : fallback;
}

async function registerPasskey(button: HTMLButtonElement): Promise<void> {
  const wasDisabled = button.disabled;
  let keepDisabled = false;
  button.disabled = true;

  try {
    if (!window.PublicKeyCredential || !navigator.credentials?.create) {
      showPasskeyError("Passkeys are not supported by this browser.");
      return;
    }

    const data = await postJSON<PasskeyOptionsResponse>("/auth/passkeys/register/options");
    const publicKey = normalizeCreationOptions(
      data.options.publicKey as PublicKeyCredentialCreationOptions,
    );

    const credential = (await navigator.credentials.create({ publicKey })) as PublicKeyCredential | null;

    if (!credential) {
      showPasskeyError("Passkey setup was cancelled.");
      return;
    }

    const responseMode =
      button.dataset.passkeyRedirect === "true" ? "json" : "linked-providers-section";
    const finish = await postRegistrationFinish({
      challenge_id: data.challenge_id,
      credential: serializeRegistrationCredential(credential),
      platform_hint: platformHint(),
      return_to: button.dataset.returnTo || "/auth/account",
    }, responseMode);

    if (button.dataset.passkeyRedirect === "true") {
      window.location.href = finish.return_to || button.dataset.returnTo || "/";
      return;
    }

    if (!finish.linkedProvidersHTML || !replaceLinkedProvidersSection(finish.linkedProvidersHTML)) {
      window.location.href = "/auth/account";
      return;
    }

    showToast("success", "Passkey added.");
  } catch (err) {
    if (err instanceof DOMException && err.name === "InvalidStateError") {
      keepDisabled = true;
      disableDuplicateButton(button);
      showPasskeyError("This device already has a passkey for this account.");
      return;
    }
    showPasskeyError(
      passkeyErrorMessage(err, "Could not add passkey.", "Passkey setup was cancelled."),
    );
  } finally {
    if (!keepDisabled) {
      button.disabled = wasDisabled;
    }
  }
}

async function loginWithPasskey(button: HTMLButtonElement): Promise<void> {
  const wasDisabled = button.disabled;
  button.disabled = true;

  try {
    if (!window.PublicKeyCredential || !navigator.credentials?.get) {
      showPasskeyError("Passkeys are not supported by this browser.");
      return;
    }

    const returnTo = button.dataset.returnTo || "/";
    const data = await postJSON<PasskeyOptionsResponse>(
      `/auth/passkeys/authenticate/options?return_to=${encodeURIComponent(returnTo)}`,
    );
    const publicKey = normalizeRequestOptions(
      data.options.publicKey as PublicKeyCredentialRequestOptions,
    );

    const credential = (await navigator.credentials.get({
      publicKey,
      mediation: data.options.mediation,
    } as CredentialRequestOptions)) as PublicKeyCredential | null;

    if (!credential) {
      showPasskeyError("Passkey sign-in was cancelled.");
      return;
    }

    const finish = await postJSON<PasskeyFinishResponse>(
      `/auth/passkeys/authenticate/finish?return_to=${encodeURIComponent(returnTo)}`,
      {
        challenge_id: data.challenge_id,
        credential: serializeAuthenticationCredential(credential),
        return_to: returnTo,
      },
    );

    window.location.href = finish.return_to || returnTo;
  } catch (err) {
    showPasskeyError(
      passkeyErrorMessage(err, "Passkey sign-in failed.", "Passkey sign-in was cancelled."),
    );
  } finally {
    button.disabled = wasDisabled;
  }
}

export function initPasskeys(root: ParentNode = document): void {
  root.querySelectorAll<HTMLButtonElement>("[data-passkey-register]").forEach((button) => {
    if (button.dataset.passkeyBound === "true") return;
    button.dataset.passkeyBound = "true";
    button.addEventListener("click", () => void registerPasskey(button));
  });

  root.querySelectorAll<HTMLButtonElement>("[data-passkey-login]").forEach((button) => {
    if (button.dataset.passkeyBound === "true") return;
    button.dataset.passkeyBound = "true";
    button.addEventListener("click", () => void loginWithPasskey(button));
  });
}
