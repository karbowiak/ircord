#!/bin/sh
set -eu
cd "$(dirname "$0")/../.."
mkdir -p .data/ergo
if [ ! -f .data/ergo/ircd.yaml ]; then
  docker run --rm --entrypoint cat ghcr.io/ergochat/ergo:v2.19.1@sha256:ef885e44f7fa19101bbbc41baef11dc280dc8107465dccaf6f0860f41b48a682 /ircd-bin/default.yaml > .data/ergo/default.yaml
  (cd backend && go run ./cmd/ergo-config < ../.data/ergo/default.yaml) > .data/ergo/ircd.yaml.tmp
  mv .data/ergo/ircd.yaml.tmp .data/ergo/ircd.yaml
fi
docker compose -f infra/ergo/compose.yaml up -d --wait
