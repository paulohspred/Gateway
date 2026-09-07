#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BUILD="$ROOT_DIR/scripts/build-release.sh"
RELEASE="$ROOT_DIR/scripts/install-release.sh"
MONITOR="$ROOT_DIR/scripts/install-rc-monitor.sh"
FRONTEND="$ROOT_DIR/scripts/install-rc-frontend.sh"
RAPID_API="$ROOT_DIR/scripts/configure-rapid-web-api.sh"
LAB="$ROOT_DIR/scripts/install-rc-lab-stack.sh"
ACCEPT="$ROOT_DIR/scripts/rc-frontend-acceptance.sh"

for file in "$BUILD" "$RELEASE" "$MONITOR" "$FRONTEND" "$RAPID_API" "$LAB" "$ACCEPT"; do
  [[ -f "$file" ]] || { echo "ERRO: contrato referencia arquivo ausente: $file" >&2; exit 1; }
done

# Supply chain / release payload.
grep -Fq 'frontend/package-lock.json' "$BUILD"
grep -Fq 'npm ci --no-audit --no-fund' "$BUILD"
grep -Fq 'component=rc-monitor-frontend' "$BUILD"
grep -Fq 'install-rc-lab-stack.sh' "$BUILD"
grep -Fq 'cp -R "$FRONTEND_DIR/dist/." "$stage/frontend/"' "$BUILD"
grep -Fq '[[ -x "$pkg/bin/rc-monitor" ]]' "$RELEASE"
grep -Fq '[[ -f "$pkg/frontend/index.html" ]]' "$RELEASE"
grep -Fq "! -path '*/bin/rc-monitor'" "$RELEASE"
echo "application release payload contract: OK"

# RC Monitor install must not relocate relative Rapid/profile paths silently and
# must restore the previous known-good state on health-gate failure.
grep -Fq 'deve ser absoluto para instalação em /etc' "$MONITOR"
grep -Fq 'nova configuração do RC Monitor não ficou ready; restaurando estado anterior' "$MONITOR"
grep -Fq 'cp --preserve=mode,ownership,timestamps "$backup_dir/config" "$CONFIG_TARGET"' "$MONITOR"
echo "RC Monitor relocation and rollback contract: OK"

# Rapid Web API: authentication read-only only. Command API must stay disabled.
grep -Fq 'AllowAuthApi' "$RAPID_API"
grep -Fq 'AllowCommandApi' "$RAPID_API"
grep -Fq 'rb'"'"'\1true\2'"'"'' "$RAPID_API"
grep -Fq 'rb'"'"'\1false\2'"'"'' "$RAPID_API"
grep -Fq 'RAPID WEB API OK: auth=true command=false' "$RAPID_API"
echo "Rapid Web API read-only contract: OK"

# Frontend serving is from the packaged release; browser never proxies to Rapid.
grep -Fq 'FRONTEND_ROOT="$ROOT/current/frontend"' "$FRONTEND"
grep -Fq 'location /api/' "$FRONTEND"
grep -Fq 'proxy_pass http://$MONITOR;' "$FRONTEND"
if grep -Fq 'proxy_pass http://127.0.0.1:10008' "$FRONTEND"; then
  echo "ERRO: frontend installer expõe Rapid Web pelo Nginx" >&2
  exit 1
fi
grep -Fq 'public, immutable' "$FRONTEND"
grep -Fq 'Content-Security-Policy' "$FRONTEND"
grep -Fq 'systemctl restart nginx.service' "$FRONTEND"
grep -Fq 'restaurando site anterior' "$FRONTEND"
echo "frontend Nginx isolation/cache/rollback contract: OK"

# Full lab flow must remain explicitly synthetic and read-only.
grep -Fq 'SIMULATION_TEST_ONLY' "$LAB"
grep -Fq 'production_validated=false' "$LAB"
grep -Fq 'rapid_commands=false' "$LAB"
grep -Fq 'physical_controller=false' "$LAB"
grep -Fq 'configure-rapid-web-api.sh' "$LAB"
grep -Fq 'rc-monitor.rapid-demo-lab.json' "$LAB"
grep -Fq 'rc-frontend-acceptance.sh' "$LAB"
echo "lab orchestration safety contract: OK"

# Deployed acceptance must prove browser-facing read-only semantics and internal
# listener isolation rather than only service process status.
grep -Fq 'only GET is allowed' "$ACCEPT"
grep -Fq 'Cache-Control: no-store' "$ACCEPT"
grep -Fq 'immutable' "$ACCEPT"
grep -Fq '10008 18080 18100' "$ACCEPT"
echo "deployed frontend acceptance contract: OK"

echo "APPLICATION INSTALLER CONTRACT PASSED"
