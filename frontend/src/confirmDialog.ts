export type ConfirmTheme = "neutral" | "danger";

export type ConfirmDialogState = {
  open: boolean;
  headline: string;
  body: string;
  confirmLabel: string;
  confirmFormId: string;
  theme: ConfirmTheme;
  submitting: boolean;
};

export type OpenConfirmOptions = {
  headline: string;
  body: string;
  confirmLabel?: string;
  confirmFormId: string;
  theme?: ConfirmTheme;
};

export type ConfirmDialogController = {
  confirm: ConfirmDialogState;
  openConfirm: (opts: OpenConfirmOptions) => void;
  closeConfirm: () => void;
  runConfirm: () => void;
};

declare global {
  interface Window {
    confirmDialog: () => ConfirmDialogController;
    htmx: any;
  }
}

document.addEventListener("alpine:init", () => {
  window.confirmDialog = function (): ConfirmDialogController {
    const closeDelayMs = 180;
    let resetTimer: number | undefined;

    function resetConfirm(confirm: ConfirmDialogState) {
      confirm.headline = "";
      confirm.body = "";
      confirm.confirmLabel = "Confirm";
      confirm.confirmFormId = "";
      confirm.theme = "neutral";
      confirm.submitting = false;
    }

    return {
      confirm: {
        open: false,
        headline: "",
        body: "",
        confirmLabel: "Confirm",
        confirmFormId: "",
        theme: "neutral",
        submitting: false,
      },

      openConfirm({
        headline,
        body,
        confirmLabel = "Confirm",
        confirmFormId,
        theme = "neutral",
      }: OpenConfirmOptions) {
        window.clearTimeout(resetTimer);
        this.confirm.open = true;
        this.confirm.headline = headline;
        this.confirm.body = body;
        this.confirm.confirmLabel = confirmLabel;
        this.confirm.confirmFormId = confirmFormId;
        this.confirm.theme = theme;
        this.confirm.submitting = false;
      },

      closeConfirm() {
        if (this.confirm.submitting) return;

        this.confirm.open = false;
        window.clearTimeout(resetTimer);
        resetTimer = window.setTimeout(() => {
          resetConfirm(this.confirm);
        }, closeDelayMs);
      },

      runConfirm() {
        if (this.confirm.submitting) return;
        if (!this.confirm.confirmFormId) return;

        const form = document.getElementById(
          this.confirm.confirmFormId,
        ) as HTMLFormElement | null;

        if (!form) {
          console.error(
            `confirmDialog: form with id "${this.confirm.confirmFormId}" not found`,
          );
          this.closeConfirm();
          return;
        }

        this.confirm.submitting = true;
        window.clearTimeout(resetTimer);

        if (isHTMXForm(form) && window.htmx) {
          form.addEventListener(
            "htmx:afterRequest",
            () => {
              this.confirm.submitting = false;
              this.closeConfirm();
            },
            { once: true },
          );
        }

        if (typeof form.requestSubmit === "function") {
          form.requestSubmit();
        } else {
          form.submit();
        }
      },
    };
  };
});

function isHTMXForm(form: HTMLFormElement): boolean {
  return (
    form.hasAttribute("hx-post") ||
    form.hasAttribute("hx-get") ||
    form.hasAttribute("hx-put") ||
    form.hasAttribute("hx-patch") ||
    form.hasAttribute("hx-delete")
  );
}
