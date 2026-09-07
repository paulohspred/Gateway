#!/usr/bin/env bash
set -Eeuo pipefail
base="${RC_ADMIN_BASE:-http://127.0.0.1:18110}"
front="${RC_FRONTEND_BASE:-http://127.0.0.1}"
fail(){ echo "FAIL: $*" >&2; exit 1; }
pass(){ echo "PASS: $*"; }
curl -fsS --max-time 3 "$base/healthz" >/dev/null || fail "rc-admin health"
pass "rc-admin health"
code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 3 "$base/api/v1/auth/me")"; [[ "$code" == 401 ]] || fail "unauthenticated me expected 401 got $code"
pass "unauthenticated session is rejected"
code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 3 -H 'Content-Type: application/json' -d '{"username":"invalid","password":"invalid-password"}' "$front/api/v1/auth/login")"; [[ "$code" == 401 ]] || fail "Nginx auth route expected 401 got $code"
pass "Nginx routes auth API to rc-admin"
code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 3 "$front/api/v1/admin/users")"; [[ "$code" == 401 ]] || fail "admin API expected 401 got $code"
pass "admin API is protected"
ss -ltn | awk '$4 ~ /127\.0\.0\.1:18110$/ {found=1} END{exit !found}' || fail "rc-admin is not loopback-bound"
pass "rc-admin listener is loopback-only"
echo "RC ADMIN ACCEPTANCE PASSED"
