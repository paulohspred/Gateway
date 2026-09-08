#!/usr/bin/env bash
set -Eeuo pipefail

SERVICE="${RC_GATEWAY_SERVICE:-rc-gateway.service}"
ROOT="${RC_GATEWAY_ROOT:-/opt/rc-gateway}"
CONFIG="${RC_GATEWAY_CONFIG:-/etc/rc-gateway.json}"
ADMIN_URL="${RC_GATEWAY_ADMIN_URL:-http://127.0.0.1:18080}"
READY_ATTEMPTS="${RC_GATEWAY_VM_READY_ATTEMPTS:-30}"
READY_DELAY="${RC_GATEWAY_VM_READY_DELAY_SECONDS:-1}"

fail(){ echo "FAIL: $*" >&2; exit 1; }
pass(){ echo "PASS: $*"; }

[[ $EUID -eq 0 ]] || fail "VM acceptance must run as root so service properties/restart can be verified"
[[ "$READY_ATTEMPTS" =~ ^[0-9]+$ ]] && (( READY_ATTEMPTS >= 1 && READY_ATTEMPTS <= 300 )) || fail "RC_GATEWAY_VM_READY_ATTEMPTS must be 1..300"
[[ "$READY_DELAY" =~ ^[0-9]+$ ]] && (( READY_DELAY >= 1 && READY_DELAY <= 60 )) || fail "RC_GATEWAY_VM_READY_DELAY_SECONDS must be 1..60"
for cmd in systemctl curl journalctl ss; do command -v "$cmd" >/dev/null || fail "missing command: $cmd"; done
[[ -x "$ROOT/current/bin/rc-gateway" ]] || fail "installed binary missing: $ROOT/current/bin/rc-gateway"
[[ -f "$CONFIG" ]] || fail "configuration missing: $CONFIG"

"$ROOT/current/bin/rc-gateway" --version
"$ROOT/current/bin/rc-gateway" --check-config --config "$CONFIG"
pass "binary and configuration validate"

[[ "$(systemctl show -p User --value "$SERVICE")" == "rc-gateway" ]] || fail "service is not running as rc-gateway user"
[[ "$(systemctl show -p Type --value "$SERVICE")" == "notify" ]] || fail "service Type must be notify"
watchdog="$(systemctl show -p WatchdogUSec --value "$SERVICE")"
[[ "$watchdog" != "0" && "$watchdog" != "0us" && -n "$watchdog" ]] || fail "systemd watchdog is not enabled"
pass "least-privilege service identity and watchdog configured"

wait_ready(){
  local i
  for ((i=1; i<=READY_ATTEMPTS; i++)); do
    if systemctl is-active --quiet "$SERVICE" && curl -fsS --max-time 2 "$ADMIN_URL/healthz" >/dev/null && curl -fsS --max-time 2 "$ADMIN_URL/readyz" >/dev/null; then
      return 0
    fi
    sleep "$READY_DELAY"
  done
  return 1
}

wait_ready || fail "service did not become healthy/ready"
pass "healthz and readyz are healthy"

status="$(curl -fsS --max-time 2 "$ADMIN_URL/status")" || fail "status endpoint failed"
metrics="$(curl -fsS --max-time 2 "$ADMIN_URL/metrics")" || fail "metrics endpoint failed"
grep -q '"ready":true' <<<"$status" || fail "status does not report ready=true"
grep -q '^rc_gateway_ready 1$' <<<"$metrics" || fail "metrics do not report rc_gateway_ready 1"
pass "status and metrics are coherent"

before_pid="$(systemctl show -p MainPID --value "$SERVICE")"
systemctl restart "$SERVICE"
wait_ready || fail "service failed readiness after controlled restart"
after_pid="$(systemctl show -p MainPID --value "$SERVICE")"
[[ "$after_pid" =~ ^[0-9]+$ && "$after_pid" -gt 1 ]] || fail "invalid MainPID after restart"
[[ "$after_pid" != "$before_pid" ]] || echo "WARN: MainPID did not change across restart; inspect service behavior"
pass "controlled restart recovered to readiness"

if ss -lntH | awk '{print $4}' | grep -Eq '(^|\])0\.0\.0\.0:18080$|^\*:18080$|^\[::\]:18080$'; then
  fail "default admin port 18080 appears exposed on a wildcard address"
fi
pass "default admin endpoint is not wildcard-exposed"

nrestarts="$(systemctl show -p NRestarts --value "$SERVICE")"
echo "INFO: systemd NRestarts=$nrestarts"
echo "VM ACCEPTANCE BASIC GATE PASSED"
echo "Remaining: upgrade/rollback with two releases, network fault injection, resource soak and any deployment-specific HIL."
