export function showRedirecting() {
  document.documentElement.classList.add("is-redirecting");
}

export function hideRedirecting() {
  document.documentElement.classList.remove("is-redirecting");
}
