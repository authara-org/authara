import { lockModalScroll, unlockModalScroll } from "./modalScrollLock";

const scrollLockOwner = "account-password-dialog";

function dialogElements(): {
  dialog: HTMLDialogElement;
  content: HTMLElement;
} | null {
  const dialog = document.querySelector<HTMLDialogElement>(
    "#account-password-dialog",
  );
  const content = document.querySelector<HTMLElement>(
    "#account-password-dialog-content",
  );
  return dialog && content ? { dialog, content } : null;
}

export function initAccountPasswordDialog(): void {
  const elements = dialogElements();
  if (!elements || elements.dialog.dataset.initialized === "true") return;
  elements.dialog.dataset.initialized = "true";

  elements.dialog
    .querySelector("[data-account-password-dialog-close]")
    ?.addEventListener("click", () => elements.dialog.close());
  elements.dialog.addEventListener("close", () => {
    unlockModalScroll(scrollLockOwner);
    elements.content.replaceChildren();
  });
  elements.dialog.addEventListener("click", (event) => {
    if (event.target === elements.dialog) elements.dialog.close();
  });
}

export function openAccountPasswordDialog(): void {
  const elements = dialogElements();
  if (!elements) return;
  if (!elements.dialog.open) {
    elements.dialog.showModal();
    lockModalScroll(scrollLockOwner);
  }
  elements.content.querySelector<HTMLElement>("[autofocus]")?.focus();
}

export function closeAccountPasswordDialog(): void {
  const elements = dialogElements();
  if (!elements) return;
  if (elements.dialog.open) elements.dialog.close();
  elements.content.replaceChildren();
}
