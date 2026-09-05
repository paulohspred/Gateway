#!/usr/bin/env bash
set -Eeuo pipefail

SECONDS_TO_RUN="${1:-86400}"
if ! [[ "$SECONDS_TO_RUN" =~ ^[0-9]+$ ]] || (( SECONDS_TO_RUN < 1 || SECONDS_TO_RUN > 604800 )); then
  echo "uso: $0 [segundos entre 1 e 604800]" >&2
  exit 2
fi

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

TOTAL_TIMEOUT=$((SECONDS_TO_RUN + 180))
echo "RC Gateway soak: ${SECONDS_TO_RUN}s (timeout global ${TOTAL_TIMEOUT}s)"
RC_GATEWAY_SOAK=1 \
RC_GATEWAY_SOAK_SECONDS="$SECONDS_TO_RUN" \
timeout "${TOTAL_TIMEOUT}s" go test ./internal/bridge -run '^TestSoakImpairedReconnect$' -count=1 -timeout "${TOTAL_TIMEOUT}s"
