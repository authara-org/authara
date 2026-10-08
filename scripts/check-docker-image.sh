#!/usr/bin/env sh
set -eu

image="${1:?usage: check-docker-image.sh IMAGE}"
container_id="$(docker create "$image")"
listing="$(mktemp)"

cleanup() {
  docker rm "$container_id" >/dev/null 2>&1 || true
  rm -f "$listing"
}
trap cleanup EXIT INT TERM

docker export "$container_id" | tar -tf - > "$listing"

forbidden='(^|/)(\.git|\.env([^/]*)?|node_modules|\.venv|__pycache__|\.cache|\.parcel-cache|coverage([^/]*)?|\.DS_Store|Thumbs\.db|\.idea|\.vscode|\.nvim|[^/]+\.(out|test|tmp|iml|swp|swo))(/|$)'
if grep -E "$forbidden" "$listing"; then
  echo "forbidden path found in image: $image" >&2
  exit 1
fi

printf '%s contains no forbidden paths\n' "$image"
