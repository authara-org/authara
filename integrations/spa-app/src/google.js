const googleIdentityURL = "https://accounts.google.com/gsi/client";

let googleIdentityPromise;

export function loadGoogleIdentity() {
  if (window.google?.accounts?.id) return Promise.resolve(window.google);
  if (googleIdentityPromise) return googleIdentityPromise;

  googleIdentityPromise = new Promise((resolve, reject) => {
    const script = document.createElement("script");
    script.src = googleIdentityURL;
    script.async = true;
    script.onload = () =>
      window.google?.accounts?.id
        ? resolve(window.google)
        : reject(new Error("Google authentication did not initialize."));
    script.onerror = () =>
      reject(new Error("Could not load Google authentication."));
    document.head.append(script);
  });

  return googleIdentityPromise;
}
