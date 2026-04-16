import { initVerificationCodeForm } from "./verificationInput";
import { showRedirecting, hideRedirecting } from "./ui";
import "./oauth";
import { initTheme, setTheme } from "./theme";

declare global {
  interface Window {
    htmx: any;
    autharaGoogleCallback: (response: { credential?: string }) => Promise<void>;
  }
}

window.htmx.config.allowNestedOobSwaps = false;
window.htmx.config.defaultSwapStyle = "outerHTML";
(window as any).setTheme = setTheme;

document.addEventListener("DOMContentLoaded", () => {
  initVerificationCodeForm(document);
  initTheme();
});

document.body.addEventListener("htmx:beforeSwap", function (evt: any) {
  if (
    evt.detail.xhr.status === 422 ||
    evt.detail.xhr.status === 400 ||
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

document.body.addEventListener("htmx:afterSwap", () => {
  initVerificationCodeForm(document);
});
