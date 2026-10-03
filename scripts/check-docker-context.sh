#!/usr/bin/env sh
set -eu

context="${1:?usage: check-docker-context.sh CONTEXT NAME [MAX_KIB]}"
name="${2:?usage: check-docker-context.sh CONTEXT NAME [MAX_KIB]}"
max_kib="${3:-8192}"
output_dir="$(mktemp -d)"

cleanup() {
  rm -rf "$output_dir"
}
trap cleanup EXIT INT TERM

docker buildx build \
  --no-cache \
  --progress=plain \
  --file docker/Dockerfile.context-check \
  --build-arg "MAX_CONTEXT_KIB=$max_kib" \
  --output "type=local,dest=$output_dir" \
  "$context"

size_kib="$(tr -d '[:space:]' < "$output_dir/context-size-kib")"
printf '%s build context: %s KiB (limit: %s KiB)\n' "$name" "$size_kib" "$max_kib"

if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
  printf '| `%s` | %s KiB | %s KiB |\n' \
    "$name" "$size_kib" "$max_kib" >> "$GITHUB_STEP_SUMMARY"
fi
