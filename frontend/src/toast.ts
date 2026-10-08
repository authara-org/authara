export type ToastKind = "success" | "info" | "error";

function removeToast(toast: HTMLElement): void {
  if (toast.dataset.removing === "true") return;
  toast.dataset.removing = "true";
  toast.classList.remove("toast-enter");
  toast.classList.add("toast-exit");
  toast.addEventListener("animationend", () => toast.remove(), { once: true });
}

export function showToast(kind: ToastKind, message: string): void {
  const container = document.querySelector<HTMLElement>("#toast-container");
  const template = document.querySelector<HTMLTemplateElement>(
    `#toast-template-${kind}`,
  );
  const templateRoot = template?.content.firstElementChild;

  if (!container || !(templateRoot instanceof HTMLElement)) {
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
