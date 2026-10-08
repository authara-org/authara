import { lockModalScroll, unlockModalScroll } from "./modalScrollLock";

const completionEvent = "autharaRecentAuthenticationComplete";
const scrollLockOwner = "recent-authentication-dialog";

type ActiveChallenge = {
  resolve: () => void;
  reject: (reason: Error) => void;
};

let activeChallenge: ActiveChallenge | null = null;

function dialogElements(): {
  dialog: HTMLDialogElement;
  content: HTMLElement;
  loading: HTMLElement | null;
} | null {
  const dialog = document.querySelector<HTMLDialogElement>(
    "#recent-authentication-dialog",
  );
  const content = document.querySelector<HTMLElement>(
    "#recent-authentication-content",
  );
  const loading = document.querySelector<HTMLElement>(
    "[data-recent-authentication-loading]",
  );
  return dialog && content ? { dialog, content, loading } : null;
}

function challengeURL(rawURL: string, forDialog: boolean): string {
  const url = new URL(rawURL || "/auth/reauthenticate", window.location.origin);
  if (url.origin !== window.location.origin) {
    throw new Error("Invalid authentication challenge URL.");
  }
  const challengeID = url.searchParams.get("authentication_challenge_id");
  if (!challengeID) {
    throw new Error("Authentication challenge is missing.");
  }
  if (forDialog) {
    const completion = new URL(
      "/auth/reauthenticate/complete",
      window.location.origin,
    );
    completion.searchParams.set("authentication_challenge_id", challengeID);
    completion.searchParams.set("embedded", "1");
    url.searchParams.set(
      "return_to",
      `${completion.pathname}${completion.search}`,
    );
    url.searchParams.set("embedded", "1");
  }
  return `${url.pathname}${url.search}`;
}

function beginLoadingChallenge(
  elements: NonNullable<ReturnType<typeof dialogElements>>,
): void {
  if (elements.loading) elements.loading.hidden = false;
  elements.content.replaceChildren();
  elements.content.style.minHeight = "18rem";
  elements.content.style.visibility = "hidden";
  elements.dialog.showModal();
  lockModalScroll(scrollLockOwner);
}

function revealLoadedChallenge(
  elements: NonNullable<ReturnType<typeof dialogElements>>,
): void {
  window.requestAnimationFrame(() => {
    if (elements.loading) elements.loading.hidden = true;
    elements.content.style.minHeight = "";
    elements.content.style.visibility = "";
  });
}

function resetChallengePresentation(
  elements: NonNullable<ReturnType<typeof dialogElements>>,
): void {
  if (elements.loading) elements.loading.hidden = true;
  elements.content.style.minHeight = "";
  elements.content.style.visibility = "";
}

function finishChallenge(): void {
  const elements = dialogElements();
  const active = activeChallenge;
  activeChallenge = null;
  if (elements) {
    resetChallengePresentation(elements);
    if (elements.dialog.open) elements.dialog.close();
    elements.content.replaceChildren();
  }
  unlockModalScroll(scrollLockOwner);
  active?.resolve();
}

export function notifyRecentAuthenticationComplete(): boolean {
  if (!activeChallenge) return false;
  finishChallenge();
  return true;
}

function cancelChallenge(): void {
  const elements = dialogElements();
  const active = activeChallenge;
  activeChallenge = null;
  if (elements) {
    resetChallengePresentation(elements);
    if (elements.dialog.open) elements.dialog.close();
    elements.content.replaceChildren();
  }
  unlockModalScroll(scrollLockOwner);
  active?.reject(new Error("Authentication challenge cancelled."));
}

export function requestRecentAuthentication(rawURL: string): Promise<void> {
  const elements = dialogElements();
  if (
    !elements ||
    typeof elements.dialog.showModal !== "function" ||
    typeof window.htmx?.ajax !== "function"
  ) {
    return Promise.reject(
      new Error("The recent-authentication dialog is unavailable."),
    );
  }
  if (activeChallenge) {
    return Promise.reject(
      new Error("An authentication challenge is already active."),
    );
  }
  const url = challengeURL(rawURL, true);

  return new Promise<void>((resolve, reject) => {
    activeChallenge = { resolve, reject };
    beginLoadingChallenge(elements);
    Promise.resolve(
      window.htmx.ajax("GET", url, {
        source: elements.dialog,
        target: elements.content,
        swap: "innerHTML",
      }),
    ).then(
      () => {
        if (activeChallenge) revealLoadedChallenge(elements);
      },
      (error) => {
        cancelChallenge();
        console.error("Could not load recent authentication.", error);
      },
    );
  });
}

export function initRecentAuthentication(): void {
  if (
    document.querySelector("[data-recent-authentication-complete]") &&
    notifyRecentAuthenticationComplete()
  ) {
    return;
  }

  document.body.addEventListener(completionEvent, () => {
    if (activeChallenge) {
      finishChallenge();
      return;
    }
    notifyRecentAuthenticationComplete();
  });

  const elements = dialogElements();
  if (!elements || elements.dialog.dataset.initialized === "true") return;
  elements.dialog.dataset.initialized = "true";

  elements.dialog
    .querySelector("[data-recent-authentication-cancel]")
    ?.addEventListener("click", cancelChallenge);
  elements.dialog.addEventListener("click", (event) => {
    if (event.target === elements.dialog) cancelChallenge();
  });
  elements.dialog.addEventListener("cancel", (event) => {
    event.preventDefault();
    cancelChallenge();
  });
  elements.dialog.addEventListener("close", () => {
    unlockModalScroll(scrollLockOwner);
  });
}

export function retryHTMXElement(element: Element, trigger?: string): boolean {
  if (!window.htmx) return false;
  if (element instanceof HTMLFormElement) {
    window.htmx.trigger(element, "submit");
    return true;
  }
  if (element instanceof HTMLElement) {
    window.htmx.trigger(element, trigger || "click");
    return true;
  }
  return false;
}
