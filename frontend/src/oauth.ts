import { showRedirecting } from "./ui";

function getGoogleFlowButton(): HTMLElement | null {
  return document.querySelector("[data-google-flow]");
}

function getCSRFToken(): string {
  const match = document.cookie.match(/(?:^|;\s*)authara_csrf=([^;]+)/);
  return match ? decodeURIComponent(match[1]) : "";
}

function getGoogleNonce(): string {
  const onload = document.querySelector<HTMLElement>("#g_id_onload");
  return onload?.dataset.nonce || "";
}

window.autharaGoogleCallback = async (response: { credential?: string }) => {
  const credential = response?.credential;
  if (!credential) return;

  const btn = getGoogleFlowButton();
  const flow = btn?.dataset.googleFlow || "login";
  const returnTo = btn?.dataset.returnTo || "/";
  const provider = btn?.dataset.provider || "google";

  const form = new URLSearchParams();
  form.set("credential", credential);
  form.set("flow", flow);
  form.set("nonce", getGoogleNonce());

  try {
    if (flow === "link") {
      const startRes = await fetch(
        `/auth/providers/${encodeURIComponent(provider)}/link/start`,
        {
          method: "POST",
          credentials: "include",
          headers: {
            "Content-Type": "application/x-www-form-urlencoded",
            "X-CSRF-Token": getCSRFToken(),
          },
          body: "",
        },
      );

      if (!startRes.ok) {
        window.location.href = "/auth/account";
        return;
      }

      const data = (await startRes.json()) as { link_id?: string };
      if (!data.link_id) {
        window.location.href = "/auth/account";
        return;
      }

      form.set("link_id", data.link_id);
    }

    const res = await fetch(
      `/auth/oauth/google/callback?return_to=${encodeURIComponent(returnTo)}`,
      {
        method: "POST",
        credentials: "include",
        headers: {
          "Content-Type": "application/x-www-form-urlencoded",
          "X-CSRF-Token": getCSRFToken(),
        },
        body: form.toString(),
        redirect: "manual",
      },
    );

    if (res.status >= 300 && res.status < 400) {
      const location = res.headers.get("Location");
      if (location) {
        showRedirecting();
        window.location.href = location;
        return;
      }
    }

    if (res.ok) {
      window.location.href = flow === "link" ? "/auth/account" : returnTo;
      return;
    }

    window.location.href =
      flow === "link"
        ? "/auth/account"
        : `/auth/login?return_to=${encodeURIComponent(returnTo)}`;
  } catch {
    window.location.href =
      flow === "link"
        ? "/auth/account"
        : `/auth/login?return_to=${encodeURIComponent(returnTo)}`;
  }
};
