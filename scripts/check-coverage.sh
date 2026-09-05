#!/usr/bin/env bash
set -Eeuo pipefail

PROFILE="${1:-coverage.out}"
MINIMUM="${RC_GATEWAY_MIN_COVERAGE:-58.0}"

[[ -f "$PROFILE" ]] || { echo "ERRO: perfil de cobertura ausente: $PROFILE" >&2; exit 2; }
[[ "$MINIMUM" =~ ^[0-9]+([.][0-9]+)?$ ]] || { echo "ERRO: RC_GATEWAY_MIN_COVERAGE inválido: $MINIMUM" >&2; exit 2; }

summary="$(go tool cover -func="$PROFILE" | tail -1)"
actual="$(awk '{v=$NF; gsub(/%/,"",v); print v}' <<<"$summary")"
[[ "$actual" =~ ^[0-9]+([.][0-9]+)?$ ]] || { echo "ERRO: não foi possível interpretar cobertura: $summary" >&2; exit 2; }

awk -v actual="$actual" -v minimum="$MINIMUM" 'BEGIN { exit !(actual + 0 >= minimum + 0) }' || {
  echo "ERRO: cobertura total ${actual}% abaixo do mínimo ${MINIMUM}%." >&2
  exit 1
}

printf 'Cobertura OK: %s%% >= %s%%\n' "$actual" "$MINIMUM"
