#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_DIR="$(mktemp -d)"
BIN="$TMP_DIR/rc-monitor"
LOG="$TMP_DIR/rc-monitor.log"
RESTART_CYCLES="${RC_MONITOR_RESTART_CYCLES:-20}"
PORT="${RC_MONITOR_TEST_PORT:-$((20000 + ($$ % 20000)))}"
ADDR="127.0.0.1:${PORT}"
BASE_URL="http://${ADDR}"
MONITOR_PID=""

fail() {
  echo "ERRO: $*" >&2
  if [[ -f "$LOG" ]]; then
    echo "--- rc-monitor log ---" >&2
    cat "$LOG" >&2 || true
  fi
  exit 1
}

cleanup() {
  set +e
  if [[ -n "$MONITOR_PID" ]] && kill -0 "$MONITOR_PID" 2>/dev/null; then
    kill -TERM "$MONITOR_PID" 2>/dev/null
    wait "$MONITOR_PID" 2>/dev/null
  fi
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

[[ "$RESTART_CYCLES" =~ ^[0-9]+$ ]] && (( RESTART_CYCLES >= 1 && RESTART_CYCLES <= 100 )) || \
  fail "RC_MONITOR_RESTART_CYCLES deve ser 1..100"
[[ "$PORT" =~ ^[0-9]+$ ]] && (( PORT >= 1024 && PORT <= 65535 )) || \
  fail "RC_MONITOR_TEST_PORT deve ser 1024..65535"
command -v curl >/dev/null || fail "curl não encontrado"
command -v go >/dev/null || fail "go não encontrado"

cd "$ROOT_DIR"
go build -trimpath -o "$BIN" ./cmd/rc-monitor

wait_ready() {
  local attempt
  for attempt in $(seq 1 100); do
    if curl -fsS --max-time 1 "$BASE_URL/readyz" >/dev/null 2>&1; then
      return 0
    fi
    if [[ -z "$MONITOR_PID" ]] || ! kill -0 "$MONITOR_PID" 2>/dev/null; then
      return 1
    fi
    sleep 0.05
  done
  return 1
}

start_monitor() {
  : > "$LOG"
  "$BIN" -bind "$ADDR" -provider fake >>"$LOG" 2>&1 &
  MONITOR_PID=$!
  wait_ready || fail "rc-monitor não ficou ready em $BASE_URL"

  curl -fsS --max-time 2 "$BASE_URL/healthz" | grep -q '"status":"ok"' || fail "healthz inválido"
  curl -fsS --max-time 2 "$BASE_URL/readyz" | grep -q '"status":"ready"' || fail "readyz inválido"
  curl -fsS --max-time 2 "$BASE_URL/api/v1/generators" | grep -q 'gen-sim-001' || fail "API de geradores inválida"
  curl -fsS --max-time 2 "$BASE_URL/metrics" | grep -q '^rc_monitor_uptime_seconds ' || fail "metrics inválido"
}

stop_monitor() {
  local pid="$MONITOR_PID"
  local rc
  [[ -n "$pid" ]] || fail "PID do rc-monitor ausente"
  kill -TERM "$pid"
  set +e
  wait "$pid"
  rc=$?
  set -e
  MONITOR_PID=""
  [[ $rc -eq 0 ]] || fail "rc-monitor terminou com status $rc após SIGTERM"

  if curl -fsS --max-time 1 "$BASE_URL/healthz" >/dev/null 2>&1; then
    fail "listener ainda responde após shutdown do processo"
  fi
}

for cycle in $(seq 1 "$RESTART_CYCLES"); do
  start_monitor
  stop_monitor
done

echo "RC MONITOR PROCESS RECOVERY OK: cycles=$RESTART_CYCLES address=$ADDR"
