import assert from "node:assert/strict";
import { test } from "node:test";

import { APIError } from "./api.js";
import {
  appleErrorMessage,
  authorizeWithApple,
  isAppleCancellation,
  loadAppleIdentity,
  prepareAppleButton,
} from "./apple.js";

function json(body, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

test("Apple SDK loads once, renders the official button, and authorizes with a fresh server flow", async () => {
  const originalDocument = globalThis.document;
  const originalFetch = globalThis.fetch;
  const originalWindow = globalThis.window;
  const appendedScripts = [];
  const initialized = [];
  const signIns = [];
  let renderCalls = 0;
  let signInResult = {
    authorization: { code: "apple-code", state: "authorization-state" },
  };
  const auth = {
    init(options) {
      initialized.push(options);
    },
    renderButton() {
      renderCalls += 1;
    },
    async signIn(options) {
      signIns.push(options);
      if (signInResult instanceof Error) throw signInResult;
      return signInResult;
    },
  };
  const document = {
    createElement(tag) {
      assert.equal(tag, "script");
      return {};
    },
    head: {
      append(script) {
        appendedScripts.push(script);
        queueMicrotask(() => {
          globalThis.window.AppleID = { auth };
          script.onload();
        });
      },
    },
    querySelectorAll() {
      return [];
    },
  };
  let optionCalls = 0;
  globalThis.window = {};
  globalThis.document = document;
  globalThis.fetch = async () => {
    optionCalls += 1;
    if (optionCalls === 1) {
      return json({ error: { code: "not_found", message: "disabled" } }, 404);
    }
    return json({
      client_id: "com.example.web",
      redirect_uri: "https://auth.example.com/auth/oauth/apple/callback",
      state: `state-${optionCalls}`,
      nonce: `nonce-${optionCalls}`,
    });
  };

  try {
    const firstLoad = loadAppleIdentity();
    const secondLoad = loadAppleIdentity();
    assert.equal(firstLoad, secondLoad);
    assert.equal(await firstLoad, auth);
    assert.equal(appendedScripts.length, 1);

    const target = {
      id: "",
      removeAttribute(name) {
        if (name === "id") this.id = "";
      },
    };
    await assert.rejects(
      prepareAppleButton(target),
      (error) => error instanceof APIError && error.status === 404,
    );
    await prepareAppleButton(target);
    assert.equal(renderCalls, 1);
    assert.equal(initialized.length, 1);
    assert.deepEqual(initialized[0], {
      clientId: "com.example.web",
      scope: "email",
      redirectURI: "https://auth.example.com/auth/oauth/apple/callback",
      state: "state-2",
      nonce: "nonce-2",
      usePopup: true,
    });

    const authorization = await authorizeWithApple();
    assert.deepEqual(authorization, {
      code: "apple-code",
      state: "authorization-state",
    });
    assert.equal(signIns.length, 1);
    assert.equal(signIns[0].state, "state-3");
    assert.equal(signIns[0].nonce, "nonce-3");
    assert.equal(appendedScripts.length, 1);

    const cancellation = Object.assign(new Error("cancelled"), {
      error: "user_cancelled_authorize",
    });
    signInResult = cancellation;
    await assert.rejects(
      authorizeWithApple(),
      (error) => error === cancellation,
    );
  } finally {
    globalThis.document = originalDocument;
    globalThis.fetch = originalFetch;
    globalThis.window = originalWindow;
  }
});

test("Apple SDK errors remain actionable", () => {
  assert.equal(
    isAppleCancellation({ error: "user_cancelled_authorize" }),
    true,
  );
  assert.equal(isAppleCancellation({ error: "popup_closed_by_user" }), true);
  assert.equal(isAppleCancellation({ error: "invalid_request" }), false);
  assert.equal(
    appleErrorMessage({ error: "invalid_request" }),
    "Apple authentication failed (invalid_request).",
  );
  assert.equal(
    appleErrorMessage({
      error: "invalid_request",
      message: "Invalid return URL",
    }),
    "Invalid return URL",
  );
});
