export { };

declare global {
  interface Window {
    htmx: any;
  }
}

window.htmx.config.allowNestedOobSwaps = false; // Disable nested OOB swaps
window.htmx.config.defaultSwapStyle = "outerHTML"; // Disable nested OOB swaps

document.body.addEventListener("htmx:beforeSwap", function(evt: any) {
  // Allow 422 and 400 responses to swap
  // We treat these as form validation errors
  if (
    evt.detail.xhr.status === 422 ||
    evt.detail.xhr.status === 400 ||
    evt.detail.xhr.status === 429
  ) {
    evt.detail.shouldSwap = true;
    evt.detail.isError = false;
  }
});

function showRedirecting() {
  document.documentElement.classList.add("is-redirecting");
}

function hideRedirecting() {
  document.documentElement.classList.remove("is-redirecting");
}

document.body.addEventListener("htmx:afterRequest", (e: Event) => {
  const evt = e as CustomEvent<any>;
  const xhr = evt.detail?.xhr;
  if (!xhr) return;

  const redirect = xhr.getResponseHeader("HX-Redirect");
  if (redirect) showRedirecting();
});

window.addEventListener("pageshow", hideRedirecting);
