#!/usr/bin/env bash
set -Eeuo pipefail

BASE_URL="${RC_FRONTEND_BASE_URL:-http://127.0.0.1}"
MONITOR_URL="${RC_MONITOR_BASE_URL:-http://127.0.0.1:18100}"
ADMIN_URL="${RC_ADMIN_BASE_URL:-http://127.0.0.1:18110}"
RAPID_WEB_URL="${RAPID_SCADA_WEB_URL:-http://127.0.0.1:10008}"

fail(){ echo "FAIL: $*" >&2; exit 1; }
pass(){ echo "PASS: $*"; }
for cmd in curl python3 ss; do command -v "$cmd" >/dev/null 2>&1 || fail "missing command: $cmd"; done

curl -fsS --max-time 3 "$MONITOR_URL/readyz" >/dev/null || fail "RC Monitor is not ready"
curl -fsS --max-time 3 "$ADMIN_URL/readyz" >/dev/null || fail "RC Admin is not ready"
curl -fsS --max-time 3 "$RAPID_WEB_URL/" >/dev/null || fail "Rapid Webstation loopback endpoint is unavailable"
pass "Rapid Webstation, RC Monitor and RC Admin are reachable on loopback"

for route in / /generators /alarms /events /communication /login /account/security /engineering/commissioning /admin/users; do
  headers="$(mktemp)"; body="$(mktemp)"
  code="$(curl -sS --max-time 5 -D "$headers" -o "$body" -w '%{http_code}' "$BASE_URL$route")"
  [[ "$code" == "200" ]] || fail "$route returned HTTP $code"
  grep -qi '^Content-Type: text/html' "$headers" || fail "$route is not HTML"
  rm -f "$headers" "$body"
done
pass "SPA public/deep routes return index HTML for client-side auth routing"

# Browser-facing operation API must be authenticated. This proves Nginx no longer
# bypasses rc-admin by proxying directly to rc-monitor.
body="$(mktemp)"
code="$(curl -sS --max-time 5 -o "$body" -w '%{http_code}' "$BASE_URL/api/v1/generators")"
[[ "$code" == "401" ]] || fail "unauthenticated generators expected 401 got $code"
grep -q 'authentication required' "$body" || fail "unauthenticated generators did not return rc-admin auth error"
rm -f "$body"
pass "browser-facing operation API is protected by rc-admin"

# The operational data plane remains independently read-only on loopback.
generators="$(curl -fsS --max-time 5 "$MONITOR_URL/api/v1/generators")" || fail "direct monitor generators API failed"
generator_id="$(python3 -c 'import json,sys; data=json.load(sys.stdin); print(data[0]["id"] if data else "")' <<<"$generators")"
[[ -n "$generator_id" ]] || fail "direct monitor generators API returned no generator"
code="$(curl -sS --max-time 5 -o /dev/null -w '%{http_code}' "$BASE_URL/generators/$generator_id")"
[[ "$code" == "200" ]] || fail "deep SPA route returned HTTP $code"
curl -fsS --max-time 5 "$MONITOR_URL/api/v1/system/health" >/dev/null || fail "direct monitor system health failed"
telemetry="$(curl -fsS --max-time 5 "$MONITOR_URL/api/v1/generators/$generator_id/telemetry")" || fail "direct monitor telemetry failed"
python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["generatorId"]; assert d["communication"] in {"online","offline"}; assert isinstance(d["metrics"], dict)' <<<"$telemetry" || fail "telemetry schema sanity check failed"
pass "RC Monitor read-only data plane remains healthy on loopback"

post_body="$(mktemp)"
post_code="$(curl -sS --max-time 5 -o "$post_body" -w '%{http_code}' -X POST "$MONITOR_URL/api/v1/generators")"
[[ "$post_code" == "405" ]] || fail "direct monitor POST expected 405 got $post_code"
grep -q 'only GET is allowed' "$post_body" || fail "direct monitor 405 body missing canonical message"
rm -f "$post_body"
pass "RC Monitor GET-only contract is preserved"

# Unauthenticated mutations must fail at auth before reaching either backend.
code="$(curl -sS --max-time 5 -o /dev/null -w '%{http_code}' -X POST "$BASE_URL/api/v1/generators")"
[[ "$code" == "401" ]] || fail "browser-facing unauthenticated POST expected 401 got $code"
pass "Nginx does not expose an unauthenticated write path"

index_headers="$(mktemp)"; index_body="$(mktemp)"
curl -fsS --max-time 5 -D "$index_headers" -o "$index_body" "$BASE_URL/"
grep -qi '^Cache-Control: no-store' "$index_headers" || fail "index.html does not advertise no-store"
asset="$(grep -oE '/assets/index-[A-Za-z0-9_-]+\.js' "$index_body" | head -n1 || true)"
[[ -n "$asset" ]] || fail "hashed JS asset was not found in index.html"
asset_headers="$(mktemp)"
curl -fsS --max-time 5 -D "$asset_headers" -o /dev/null "$BASE_URL$asset"
grep -qi '^Cache-Control: .*immutable' "$asset_headers" || fail "hashed asset does not advertise immutable caching"
rm -f "$index_headers" "$index_body" "$asset_headers"
pass "static cache policy is correct"

root_headers="$(mktemp)"
curl -fsS --max-time 5 -D "$root_headers" -o /dev/null "$BASE_URL/"
for header in X-Content-Type-Options X-Frame-Options Referrer-Policy Permissions-Policy Content-Security-Policy; do grep -qi "^${header}:" "$root_headers" || fail "missing HTTP security header: $header"; done
rm -f "$root_headers"
pass "HTTP security headers are present"

listeners="$(ss -lntH | awk '{print $4}')"
for port in 10008 18080 18100 18110; do
  if grep -Eq "^0\.0\.0\.0:${port}$|^\*:${port}$|^\[::\]:${port}$" <<<"$listeners"; then fail "internal port $port is wildcard-exposed"; fi
done
pass "Rapid Web, Gateway admin, RC Monitor and RC Admin are not wildcard-exposed"

echo "RC FRONTEND ACCEPTANCE PASSED"
