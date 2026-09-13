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
    <label for="email-template-text">Plain-text body</label>
    <textarea data-email-template-editor="text" id="email-template-text">Hello</textarea>
  </body>
</html>
EOF

python3 -m http.server 18765 --bind 127.0.0.1 --directory "$STATIC_DIR" \
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

grep -q 'data-editor-initialized="true"' "$TEST_ROOT/editor.html"
grep -q 'class="cm-editor' "$TEST_ROOT/editor.html"
if grep -Eq '\.js HTTP/[^ ]+" 404 ' "$TEST_ROOT/server.log"; then
  cat "$TEST_ROOT/server.log" >&2
  exit 1
fi
