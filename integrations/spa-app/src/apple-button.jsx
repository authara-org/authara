import React, { useEffect, useRef, useState } from "react";

import { APIError } from "./api.js";
import {
  appleErrorMessage,
  isAppleCancellation,
  prepareAppleButton,
} from "./apple.js";

export function AppleCredentialButton({
  disabled,
  onAction,
  onError,
  type = "continue",
}) {
  const targetRef = useRef(null);
  const actionRef = useRef(onAction);
  const errorRef = useRef(onError);
  const [available, setAvailable] = useState(true);
  const [ready, setReady] = useState(false);
  const [working, setWorking] = useState(false);

  actionRef.current = onAction;
  errorRef.current = onError;

  useEffect(() => {
    let active = true;
    prepareAppleButton(targetRef.current)
      .then(() => {
        if (active) setReady(true);
      })
      .catch((error) => {
        if (!active) return;
        if (error instanceof APIError && error.status === 404) {
          setAvailable(false);
          return;
        }
        errorRef.current(
          error.message || "Apple authentication is unavailable.",
        );
      });
    return () => {
      active = false;
    };
  }, []);

  if (!available) return null;
  return (
    <div
      className={`apple-action ${ready ? "" : "pending"} ${disabled || working ? "disabled" : ""}`}
      onClickCapture={(event) => {
        event.preventDefault();
        event.stopPropagation();
        if (!ready || disabled || working) return;
        setWorking(true);
        Promise.resolve(actionRef.current())
          .catch((error) => {
            if (!isAppleCancellation(error)) {
              errorRef.current(appleErrorMessage(error));
            }
          })
          .finally(() => setWorking(false));
      }}
    >
      <div
        ref={targetRef}
        className="apple-button"
        data-color="black"
        data-border="true"
        data-type={type}
        data-width="100%"
        data-height="100%"
      />
    </div>
  );
}
