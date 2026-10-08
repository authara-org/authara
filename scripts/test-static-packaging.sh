#!/usr/bin/env sh
set -eu

REPOSITORY_ROOT="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
TEST_ROOT="$(mktemp -d)"
STATIC_DIR="$TEST_ROOT/static"
SERVER_PID=""
BROWSER_PID=""

cleanup() {
  [ -z "$BROWSER_PID" ] || kill "$BROWSER_PID" 2>/dev/null || true
  [ -z "$SERVER_PID" ] || kill "$SERVER_PID" 2>/dev/null || true
  rm -rf "$TEST_ROOT"
}
trap cleanup EXIT INT TERM

mkdir -p "$STATIC_DIR" "$TEST_ROOT/bin"
cp -R "$REPOSITORY_ROOT/frontend/dist/." "$STATIC_DIR/"
cp -R "$REPOSITORY_ROOT/internal/http/static/." "$STATIC_DIR/"

# Compression is unrelated to this regression; avoid requiring brotli on CI hosts.
cat > "$TEST_ROOT/bin/brotli" <<'EOF'
#!/usr/bin/env sh
for file do :; done
cp "$file" "$file.br"
EOF
chmod +x "$TEST_ROOT/bin/brotli"

PATH="$TEST_ROOT/bin:$PATH" bash "$REPOSITORY_ROOT/scripts/build-static.sh" "$STATIC_DIR"
(
  cd "$STATIC_DIR"
  find . -type f -print | sort
) > "$TEST_ROOT/files.first"
cp "$STATIC_DIR/manifest.json" "$TEST_ROOT/manifest.first.json"

PATH="$TEST_ROOT/bin:$PATH" bash "$REPOSITORY_ROOT/scripts/build-static.sh" "$STATIC_DIR"
(
  cd "$STATIC_DIR"
  find . -type f -print | sort
) > "$TEST_ROOT/files.second"

cmp "$TEST_ROOT/files.first" "$TEST_ROOT/files.second"
cmp "$TEST_ROOT/manifest.first.json" "$STATIC_DIR/manifest.json"

EDITOR_CHUNK=""
for file in "$STATIC_DIR"/*.js; do
  if grep -q 'cm-overview-ruler' "$file"; then
    EDITOR_CHUNK="$(basename "$file")"
    break
  fi
done
[ -n "$EDITOR_CHUNK" ]
printf '%s\n' "$EDITOR_CHUNK" | grep -Eq '\.[0-9a-f]{16}\.js$'

APP_ASSET="$(jq -er '."app.js"' "$STATIC_DIR/manifest.json")"
HTMX_ASSET="$(jq -er '."htmx.min.js"' "$STATIC_DIR/manifest.json")"

cat > "$STATIC_DIR/editor-test.html" <<EOF
<!doctype html>
<html>
  <head>
    <meta charset="utf-8">
    <script src="$HTMX_ASSET" defer></script>
    <script src="$APP_ASSET" defer></script>
  </head>
  <body>
    <script>
      const pageLoadCount = Number(sessionStorage.getItem("authara-test-page-loads") || "0") + 1;
      sessionStorage.setItem("authara-test-page-loads", String(pageLoadCount));
      document.body.dataset.pageLoadCount = String(pageLoadCount);
    </script>
    <label for="email-template-text">Plain-text body</label>
    <textarea data-email-template-editor="text" id="email-template-text">Hello</textarea>
    <form id="sensitive-action" hx-post="/sensitive-action" hx-target="#sensitive-result" hx-swap="outerHTML"></form>
    <div id="sensitive-result"></div>
    <button id="open-password-dialog" type="button" hx-get="/password-dialog" hx-target="#account-password-dialog-content" hx-swap="innerHTML">Change password</button>
    <div id="linked-providers-section"></div>
    <dialog id="account-password-dialog">
      <button type="button" data-account-password-dialog-close>Close</button>
      <div id="account-password-dialog-content"></div>
    </dialog>
    <dialog id="recent-authentication-dialog">
      <button type="button" data-recent-authentication-cancel>Cancel</button>
      <div hidden data-recent-authentication-loading>Loading authentication…</div>
      <div id="recent-authentication-content"></div>
    </dialog>
    <script>
      const modalLockPoll = setInterval(function () {
        if (document.documentElement.classList.contains("modal-scroll-locked")) {
          document.body.dataset.modalScrollLockSeen = "true";
          clearInterval(modalLockPoll);
        }
      }, 10);
      const authenticationSkeletonPoll = setInterval(function () {
        const dialog = document.getElementById("recent-authentication-dialog");
        const loading = document.querySelector("[data-recent-authentication-loading]");
        const content = document.getElementById("recent-authentication-content");
        if (
          dialog.open &&
          !loading.hidden &&
          content.style.visibility === "hidden"
        ) {
          document.body.dataset.authenticationSkeletonSeen = "true";
          clearInterval(authenticationSkeletonPoll);
        }
      }, 1);
      document.body.addEventListener("htmx:afterSwap", function (event) {
        if (event.target.id === "sensitive-result") {
          document.getElementById("open-password-dialog").click();
        }
      });
      window.addEventListener("load", function () {
        document.getElementById("sensitive-action").requestSubmit();
      });
    </script>
  </body>
</html>
EOF

cat > "$TEST_ROOT/server.py" <<'PY'
import json
import sys
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import urlparse

static_dir = sys.argv[1]


class Handler(SimpleHTTPRequestHandler):
    sensitive_requests = 0

    def __init__(self, *args, **kwargs):
        super().__init__(*args, directory=static_dir, **kwargs)

    def send_body(self, status, content_type, body, headers=None):
        payload = body.encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(payload)))
        for key, value in (headers or {}).items():
            self.send_header(key, value)
        self.end_headers()
        self.wfile.write(payload)

    def do_POST(self):
        path = urlparse(self.path).path
        self.rfile.read(int(self.headers.get("Content-Length", "0")))
        if path == "/password-dialog-submit":
            self.send_body(
                200,
                "text/html",
                '<div id="linked-providers-section" data-password-dialog-updated="true"></div>',
                {
                    "HX-Retarget": "#linked-providers-section",
                    "HX-Reswap": "outerHTML",
                    "X-Authara-Close-Password-Dialog": "true",
                },
            )
            return
        if path != "/sensitive-action":
            self.send_error(404)
            return
        Handler.sensitive_requests += 1
        if Handler.sensitive_requests == 1:
            self.send_body(
                428,
                "application/json",
                json.dumps(
                    {
                        "reauthenticate_url": "/auth/reauthenticate?authentication_challenge_id=00000000-0000-0000-0000-000000000001"
                    }
                ),
            )
            return
        self.send_body(
            200,
            "text/html",
            '<div id="sensitive-result" data-sensitive-action-retried="true"></div>',
        )

    def do_GET(self):
        path = urlparse(self.path).path
        if path == "/password-dialog":
            self.send_body(
                200,
                "text/html",
                """<form id="password-dialog-form" hx-post="/password-dialog-submit" hx-target="#account-password-dialog-content">
  <input type="password" name="password" value="secret">
</form>
<script>setTimeout(function () {
  document.getElementById("password-dialog-form").requestSubmit();
}, 0);</script>""",
            )
            return
        if path == "/auth/reauthenticate":
            if self.headers.get("HX-Request") != "true":
                self.send_error(400, "authentication modal must request a fragment")
                return
            self.send_body(
                200,
                "text/html",
                """<div data-authentication-fragment>Authenticate</div><script>
setTimeout(function () {
  document.body.dispatchEvent(new CustomEvent("autharaRecentAuthenticationComplete"));
}, 100);
</script>""",
            )
            return
        super().do_GET()


ThreadingHTTPServer(("127.0.0.1", 18765), Handler).serve_forever()
PY

python3 "$TEST_ROOT/server.py" "$STATIC_DIR" \
  > "$TEST_ROOT/server.log" 2>&1 &
SERVER_PID=$!

attempt=0
until curl --noproxy '*' --fail --silent http://127.0.0.1:18765/editor-test.html >/dev/null; do
  attempt=$((attempt + 1))
  [ "$attempt" -lt 20 ] || exit 1
  sleep 0.1
done

if [ -n "${BROWSER_BIN:-}" ]; then
  BROWSER="$BROWSER_BIN"
elif command -v google-chrome >/dev/null 2>&1; then
  BROWSER="$(command -v google-chrome)"
elif command -v chromium >/dev/null 2>&1; then
  BROWSER="$(command -v chromium)"
elif [ -x "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" ]; then
  BROWSER="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
else
  echo "Chrome or Chromium is required for the production static test." >&2
  exit 1
fi

"$BROWSER" \
  --headless \
  --disable-background-networking \
  --disable-component-update \
  --disable-default-apps \
  --disable-extensions \
  --disable-gpu \
  --disable-sync \
  --no-first-run \
  --no-proxy-server \
  --no-sandbox \
  --user-data-dir="$TEST_ROOT/chrome" \
  --virtual-time-budget=5000 \
  --dump-dom \
  http://127.0.0.1:18765/editor-test.html \
  > "$TEST_ROOT/editor.html" 2> "$TEST_ROOT/browser.log" &
BROWSER_PID=$!

(
  sleep 15
  kill "$BROWSER_PID" 2>/dev/null || true
) &
WATCHDOG_PID=$!

wait "$BROWSER_PID" 2>/dev/null || true
BROWSER_PID=""
kill "$WATCHDOG_PID" 2>/dev/null || true
wait "$WATCHDOG_PID" 2>/dev/null || true

assert_dom_contains() {
  pattern="$1"
  description="$2"
  if ! grep -q "$pattern" "$TEST_ROOT/editor.html"; then
    echo "Production static test did not observe: $description." >&2
    grep -Eo '<(html|body|dialog)[^>]*>' "$TEST_ROOT/editor.html" >&2 || true
    exit 1
  fi
}

assert_dom_contains 'data-editor-initialized="true"' "editor initialization"
assert_dom_contains 'class="cm-editor' "CodeMirror rendering"
assert_dom_contains 'data-sensitive-action-retried="true"' "sensitive-action retry"
assert_dom_contains 'data-password-dialog-updated="true"' "password-dialog update"
assert_dom_contains 'data-modal-scroll-lock-seen="true"' "modal scroll locking"
assert_dom_contains 'data-authentication-skeleton-seen="true"' "authentication loading state"
assert_dom_contains 'data-page-load-count="1"' "single page load"
if grep -Eq '<(html|body)[^>]*class="[^"]*modal-scroll-locked' "$TEST_ROOT/editor.html"; then
  echo "Page remained scroll-locked after dialogs closed." >&2
  grep -Eo '<(html|body|dialog)[^>]*>' "$TEST_ROOT/editor.html" >&2 || true
  exit 1
fi
if grep -q 'name="password"' "$TEST_ROOT/editor.html"; then
  echo "Password dialog retained sensitive form fields after success." >&2
  exit 1
fi
if grep -Eq '\.js HTTP/[^ ]+" 404 ' "$TEST_ROOT/server.log"; then
  cat "$TEST_ROOT/server.log" >&2
  exit 1
fi
