#!/usr/bin/env sh
set -eux

STATIC_DIR="${1:-/app/internal/http/static}"
cd "$STATIC_DIR"

MANIFEST="manifest.json"
[ -s "$MANIFEST" ] || printf '{}' > "$MANIFEST"

fingerprint_extensions() {
  find . -type f \
    \( \
      -name "*.js" \
      -o -name "*.css" \
      -o -name "*.woff" \
      -o -name "*.woff2" \
      -o -name "*.ttf" \
      -o -name "*.svg" \
    \) \
    ! -name "*.br" \
    ! -name "*.gz" \
    ! -name "$MANIFEST" \
    -print0
}

compress_extensions() {
  find . -type f \
    \( \
      -name "*.js" \
      -o -name "*.css" \
      -o -name "*.html" \
      -o -name "*.svg" \
      -o -name "*.json" \
      -o -name "*.txt" \
    \) \
    ! -name "$MANIFEST" \
    -print0
}

# -----------------------------------------------------------------------------
# Fingerprint: rename files to include a content hash and record mapping in manifest
# -----------------------------------------------------------------------------

fingerprint_extensions |
while IFS= read -r -d '' path; do
  original="${path#./}"

  hash="$(
    sha256sum "$original" \
    | awk '{print $1}' \
    | cut -c1-16
  )"

  base="${original%.*}"
  ext="${original##*.}"
  fingerprinted="${base}.${hash}.${ext}"

  mv -f -- "$original" "$fingerprinted"

  tmp="$(mktemp)"
  jq \
    --arg key "$original" \
    --arg val "$fingerprinted" \
    '. + {($key): $val}' \
    "$MANIFEST" > "$tmp"
  mv -f -- "$tmp" "$MANIFEST"
done

# -----------------------------------------------------------------------------
# Compress: generate .br and .gz alongside originals (keeps original files)
# -----------------------------------------------------------------------------

compress_extensions |
xargs -0 -I{} sh -c '
  file="$1"
  brotli -f -q 11 "$file"
  gzip -kf -9 "$file"
' sh {}
