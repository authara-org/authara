import React, { useEffect, useRef, useState } from "react";

import { APIError } from "./api.js";
import {
  authorizeWithMicrosoft,
  isMicrosoftCancellation,
  prepareMicrosoft,
} from "./microsoft.js";

function MicrosoftLogo() {
  return (
    <span className="microsoft-logo" aria-hidden="true">
      <span />
      <span />
      <span />
      <span />
    </span>
  );
}

function label(type) {
  if (type === "sign-in") return "Sign in with Microsoft";
  if (type === "sign-up") return "Sign up with Microsoft";
  if (type === "link") return "Link Microsoft";
  return "Continue with Microsoft";
}

export function MicrosoftCredentialButton({
  disabled = false,
  type = "continue",
  onCredential,
  onError,
}) {
  const credentialHandler = useRef(onCredential);
  const errorHandler = useRef(onError);
  const [available, setAvailable] = useState(true);
  const [ready, setReady] = useState(false);
  const [working, setWorking] = useState(false);

  credentialHandler.current = onCredential;
  errorHandler.current = onError;

  useEffect(() => {
    let active = true;
    prepareMicrosoft()
      .then(() => {
        if (active) setReady(true);
      })
      .catch((error) => {
        if (!active) return;
        if (error instanceof APIError && error.status === 404) {
          setAvailable(false);
          return;
        }
        errorHandler.current?.(
          error.message || "Microsoft authentication is unavailable.",
        );
      });
    return () => {
      active = false;
    };
  }, []);

  if (!available) return null;

  return (
    <button
      className={`button secondary wide microsoft-action ${ready ? "" : "pending"}`}
      type="button"
      disabled={disabled || working || !ready}
      onClick={async () => {
        setReady(false);
        setWorking(true);
        const authorization = authorizeWithMicrosoft();
        try {
          await credentialHandler.current(await authorization);
        } catch (error) {
          if (!isMicrosoftCancellation(error)) {
            errorHandler.current?.(
              error.message || "Microsoft authentication failed.",
            );
          }
        } finally {
          setWorking(false);
          prepareMicrosoft()
            .then(() => setReady(true))
            .catch((error) => {
              errorHandler.current?.(
                error.message || "Microsoft authentication is unavailable.",
              );
            });
        }
      }}
    >
      <MicrosoftLogo />
      {label(type)}
    </button>
  );
}
