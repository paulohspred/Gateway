#!/usr/bin/env bash
set -Eeuo pipefail

GATEWAY_SERVICE="${RC_GATEWAY_SERVICE:-rc-gateway.service}"
ADMIN_URL="${RC_GATEWAY_ADMIN_URL:-http://127.0.0.1:18080}"
SCADA_SERVICE="${RAPID_SCADA_COMM_SERVICE:-scadacomm6.service}"
REQUIRE_SCADA_SERVICE="${RAPID_SCADA_REQUIRE_LOCAL_SERVICE:-1}"
EXPECT_SESSION="${RAPID_SCADA_EXPECT_SESSION:-1}"
WAIT_SECONDS="${RAPID_SCADA_WAIT_SECONDS:-60}"

case "$REQUIRE_SCADA_SERVICE" in
  0|1) ;;
  *) echo "ERRO: RAPID_SCADA_REQUIRE_LOCAL_SERVICE deve ser 0 ou 1." >&2; exit 64 ;;
esac
case "$EXPECT_SESSION" in
  0|1) ;;
  *) echo "ERRO: RAPID_SCADA_EXPECT_SESSION deve ser 0 ou 1." >&2; exit 64 ;;
esac
[[ "$WAIT_SECONDS" =~ ^[0-9]+$ ]] && (( WAIT_SECONDS >= 1 && WAIT_SECONDS <= 600 )) || {
  echo "ERRO: RAPID_SCADA_WAIT_SECONDS deve ser 1..600." >&2
  exit 64
}

for cmd in curl systemctl sed grep; do
  command -v "$cmd" >/dev/null 2>&1 || {
    echo "ERRO: comando obrigatório ausente: $cmd" >&2
    exit 69
  }
done

systemctl is-active --quiet "$GATEWAY_SERVICE" || {
  echo "ERRO: $GATEWAY_SERVICE não está ativo." >&2
  exit 1
}

curl -fsS --max-time 3 "$ADMIN_URL/healthz" >/dev/null
curl -fsS --max-time 3 "$ADMIN_URL/readyz" >/dev/null

if [[ "$REQUIRE_SCADA_SERVICE" == "1" ]]; then
  systemctl is-active --quiet "$SCADA_SERVICE" || {
    echo "ERRO: $SCADA_SERVICE não está ativo." >&2
    exit 1
  }
fi

status="$(curl -fsS --max-time 3 "$ADMIN_URL/status")"
if ! grep -Eq '"commandPlane"[[:space:]]*:[[:space:]]*"disabled"' <<<"$status"; then
  echo "ERRO: status do Gateway não confirma command plane desabilitado." >&2
  exit 1
fi

active_sessions() {
  local payload
  payload="$(curl -fsS --max-time 3 "$ADMIN_URL/status")"
  sed -n 's/.*"activeSessions"[[:space:]]*:[[:space:]]*\([0-9][0-9]*\).*/\1/p' <<<"$payload"
}

if [[ "$EXPECT_SESSION" == "1" ]]; then
  deadline=$((SECONDS + WAIT_SECONDS))
  sessions=0

  while (( SECONDS < deadline )); do
    value="$(active_sessions || true)"
    if [[ "$value" =~ ^[0-9]+$ ]]; then
      sessions="$value"
      if (( sessions > 0 )); then
        break
      fi
    fi
    sleep 1
  done

  if (( sessions < 1 )); then
    echo "ERRO: nenhuma sessão Rapid SCADA/Gateway apareceu em ${WAIT_SECONDS}s." >&2
    echo "Verifique Host/TcpPort, ConnectionMode, TransMode e a communication line do Rapid SCADA." >&2
    exit 1
  fi
fi

status="$(curl -fsS --max-time 3 "$ADMIN_URL/status")"
printf 'Rapid SCADA integration acceptance OK\n'
printf 'Gateway service: %s\n' "$GATEWAY_SERVICE"
if [[ "$REQUIRE_SCADA_SERVICE" == "1" ]]; then
  printf 'Rapid SCADA Communicator: %s\n' "$SCADA_SERVICE"
else
  printf 'Rapid SCADA Communicator: remote/not checked locally\n'
fi
printf 'Gateway status: %s\n' "$status"
