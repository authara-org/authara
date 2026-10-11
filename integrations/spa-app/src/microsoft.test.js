import assert from "node:assert/strict";
import test from "node:test";

import { isMicrosoftCancellation } from "./microsoft.js";

test("Microsoft popup cancellation is not reported as an authentication error", () => {
  assert.equal(isMicrosoftCancellation({ errorCode: "user_cancelled" }), true);
  assert.equal(isMicrosoftCancellation({ code: "empty_window_error" }), true);
  assert.equal(
    isMicrosoftCancellation({ errorCode: "invalid_request" }),
    false,
  );
});
