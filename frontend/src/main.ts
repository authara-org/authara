import { initVerificationCodeForm } from "./verificationInput";
import { showRedirecting, hideRedirecting } from "./ui";
import "./oauth";
import { initPasskeys } from "./passkeys";
import { initTheme, setTheme } from "./theme";
import "./confirmDialog";
import "./dropdown";
import {
  closeAccountPasswordDialog,
  initAccountPasswordDialog,
  openAccountPasswordDialog,
} from "./accountPasswordDialog";
import {
  initRecentAuthentication,
  requestRecentAuthentication,
  retryHTMXElement,
} from "./recentAuthentication";

declare global {
  interface Window {
    htmx: any;
  }
}

window.htmx.config.allowNestedOobSwaps = false;
window.htmx.config.defaultSwapStyle = "outerHTML";
(window as any).setTheme = setTheme;

const htmxRequestSources = new WeakMap<XMLHttpRequest, Element>();

function initEmailTemplateEditors(root: ParentNode) {
  if (!root.querySelector("textarea[data-email-template-editor]")) return;
  void import("./emailTemplateEditor").then((module) =>
    module.initEmailTemplateEditors(root),
  );
}

document.addEventListener("DOMContentLoaded", () => {
  initVerificationCodeForm(document);
  initPasskeys(document);
  initEmailTemplateEditors(document);
  initTheme();
  initRecentAuthentication();
  initAccountPasswordDialog();
});

document.body.addEventListener("htmx:beforeRequest", (event: Event) => {
  const evt = event as CustomEvent<any>;
  const xhr = evt.detail?.xhr;
  const source = evt.detail?.elt || evt.target;
  if (xhr instanceof XMLHttpRequest && source instanceof Element) {
    htmxRequestSources.set(xhr, source);
  }
});

document.body.addEventListener("htmx:beforeSwap", function (evt: any) {
  if (evt.detail.xhr.status === 428) {
    evt.detail.shouldSwap = false;
    evt.detail.isError = false;
    try {
      const body = JSON.parse(evt.detail.xhr.responseText) as {
        reauthenticate_url?: string;
      };
      const element = htmxRequestSources.get(evt.detail.xhr);
      if (!element) {
        throw new Error("Could not preserve the sensitive action.");
      }
      const trigger = evt.detail.requestConfig?.triggeringEvent?.type as
        | string
        | undefined;
      if (!body.reauthenticate_url) {
        throw new Error("The authentication response is missing its URL.");
      }
      void requestRecentAuthentication(body.reauthenticate_url)
        .then(() => {
          if (!retryHTMXElement(element, trigger)) {
            throw new Error("Could not retry the sensitive action with HTMX.");
          }
        })
        .catch((error) => {
          console.error("Recent authentication failed.", error);
        });
    } catch (error) {
      console.error("Could not start recent authentication.", error);
    }
    return;
  }
  if (
    evt.detail.xhr.status === 422 ||
    evt.detail.xhr.status === 400 ||
    evt.detail.xhr.status === 403 ||
    evt.detail.xhr.status === 409 ||
    evt.detail.xhr.status === 429
  ) {
    evt.detail.shouldSwap = true;
    evt.detail.isError = false;
  }
});

document.body.addEventListener("htmx:afterRequest", (e: Event) => {
  const evt = e as CustomEvent<any>;
  const xhr = evt.detail?.xhr;
  if (!xhr) return;

  const redirect = xhr.getResponseHeader("HX-Redirect");
  if (redirect) showRedirecting();
});

window.addEventListener("pageshow", hideRedirecting);

document.body.addEventListener("htmx:afterSwap", (event: Event) => {
  const evt = event as CustomEvent<any>;
  initVerificationCodeForm(document);
  initPasskeys(document);
  initEmailTemplateEditors(document);
  initAccountPasswordDialog();
  if (
    evt.detail?.xhr?.getResponseHeader("X-Authara-Close-Password-Dialog") ===
    "true"
  ) {
    closeAccountPasswordDialog();
    return;
  }
  if (
    event.target instanceof HTMLElement &&
    event.target.id === "account-password-dialog-content"
  ) {
    openAccountPasswordDialog();
  }
});
