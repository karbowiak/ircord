#!/bin/sh
# Starts only Go; the frontend is supervised separately.
set -eu
cd "$(dirname "$0")/.."
mkdir -p .data/bin
(cd backend && go build -o ../.data/bin/ircord ./cmd/ircord)
export IRCORD_DATA_DIR="${IRCORD_DATA_DIR:-$(pwd)/.data/live}"
export IRCORD_LISTEN="${IRCORD_LISTEN:-127.0.0.1:8080}"
export IRCORD_PUBLIC_URL="${IRCORD_PUBLIC_URL:-http://127.0.0.1:3000}"
export IRCORD_TRUSTED_PROXIES="${IRCORD_TRUSTED_PROXIES:-127.0.0.1/32,::1/128}"
export IRCORD_LINK_PREVIEWS="${IRCORD_LINK_PREVIEWS:-true}"
if [ -z "${IRCORD_REGISTRATION_KEY:-}" ] && [ -z "${IRCORD_REGISTRATION_KEY_FILE:-}" ] && [ -f .data/live/registration-key ]; then
  export IRCORD_REGISTRATION_KEY_FILE="$(pwd)/.data/live/registration-key"
  export IRCORD_REQUIRE_REGISTRATION_KEY=true
fi
if [ -z "${IRCORD_NETWORK_PASSWORDS:-}" ] && [ -z "${IRCORD_NETWORK_PASSWORDS_FILE:-}" ] && [ -f .data/live/network-passwords.json ]; then
  export IRCORD_NETWORK_PASSWORDS_FILE="$(pwd)/.data/live/network-passwords.json"
fi
exec .data/bin/ircord
