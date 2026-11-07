#!/bin/sh
# Generate the standalone Go client from the backend's authoritative HTTP schema.
set -eu
cd "$(dirname "$0")"
artifact=internal/contract/admin.gen.go
generated=$(mktemp "${TMPDIR:-/tmp}/ircord-cli-contract.XXXXXX")
trap 'rm -f "$generated"' EXIT HUP INT TERM
go tool oapi-codegen -config oapi-codegen.yaml -o "$generated" ../backend/openapi.json
if [ ! -s "$generated" ]; then
  echo 'CLI contract generator produced no output.' >&2
  exit 1
fi
if [ "${1:-}" = "--check" ]; then
  if ! cmp -s "$generated" "$artifact"; then
    echo 'Stale standalone CLI contract; run make api-generate.' >&2
    exit 1
  fi
else
  cp "$generated" "$artifact"
fi
