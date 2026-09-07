#!/usr/bin/env bash
set -Eeuo pipefail
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
usage(){ cat <<'USAGE'
Uso: build-release.sh [VERSION]

Variáveis: ARCHES="amd64 arm64", REQUIRE_SBOM=0|1, DIST_DIR, GO_BIN, CYCLONEDX_GOMOD_BIN.
VERSION deve conter apenas letras, números, ponto, underscore, + ou -.
USAGE
}
if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then usage; exit 0; fi
[[ $# -le 1 ]] || { echo "ERRO: argumentos em excesso" >&2; usage >&2; exit 64; }
VERSION="${1:-${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || git rev-parse --short HEAD)}}"
VERSION="${VERSION//\//-}"
[[ "$VERSION" =~ ^[A-Za-z0-9._+-]+$ ]] || { echo "ERRO: versão inválida: $VERSION" >&2; exit 2; }
COMMIT="${COMMIT:-$(git rev-parse HEAD 2>/dev/null || printf unknown)}"
SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH:-$(git show -s --format=%ct HEAD 2>/dev/null || date +%s)}"
BUILD_DATE="${BUILD_DATE:-$(date -u -d "@$SOURCE_DATE_EPOCH" +%Y-%m-%dT%H:%M:%SZ)}"
ARCHES="${ARCHES:-amd64 arm64}"
DIST_DIR="${DIST_DIR:-$ROOT_DIR/dist}"
REQUIRE_SBOM="${REQUIRE_SBOM:-0}"
NODE_VERSION_FILE="$ROOT_DIR/.node-version"
FRONTEND_DIR="$ROOT_DIR/frontend"
CYCLONEDX_GOMOD_VERSION="07257d5b9cbd2a3d4338a880c0ca50081e1ac445"
BUILD_TOOL_DIR=""

cleanup_tools(){
  [[ -z "$BUILD_TOOL_DIR" ]] || rm -rf "$BUILD_TOOL_DIR"
}
trap cleanup_tools EXIT

resolve_go(){
  GO_BIN="${GO_BIN:-}"
  if [[ -z "$GO_BIN" ]]; then
    GO_BIN="$(command -v go 2>/dev/null || true)"
  fi
  if [[ -z "$GO_BIN" && -x /usr/local/go/bin/go ]]; then
    GO_BIN=/usr/local/go/bin/go
  fi
  [[ -n "$GO_BIN" && -x "$GO_BIN" ]] || {
    echo "ERRO: Go ausente; configure GO_BIN ou instale Go. Também foi verificado /usr/local/go/bin/go." >&2
    exit 69
  }
  export PATH="$(dirname "$GO_BIN"):$PATH"
}

resolve_cyclonedx(){
  CYCLONEDX_GOMOD_BIN="${CYCLONEDX_GOMOD_BIN:-}"
  if [[ -z "$CYCLONEDX_GOMOD_BIN" ]]; then
    CYCLONEDX_GOMOD_BIN="$(command -v cyclonedx-gomod 2>/dev/null || true)"
  fi
  if [[ -n "$CYCLONEDX_GOMOD_BIN" && -x "$CYCLONEDX_GOMOD_BIN" ]]; then
    return 0
  fi
  CYCLONEDX_GOMOD_BIN=""
  if [[ "$REQUIRE_SBOM" != "1" ]]; then
    return 0
  fi

  BUILD_TOOL_DIR="$(mktemp -d)"
  echo "cyclonedx-gomod ausente; instalando versão pinada $CYCLONEDX_GOMOD_VERSION em diretório temporário..." >&2
  if ! GOBIN="$BUILD_TOOL_DIR" "$GO_BIN" install "github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@$CYCLONEDX_GOMOD_VERSION"; then
    echo "ERRO: não foi possível instalar cyclonedx-gomod pinado para gerar SBOM." >&2
    exit 69
  fi
  CYCLONEDX_GOMOD_BIN="$BUILD_TOOL_DIR/cyclonedx-gomod"
  [[ -x "$CYCLONEDX_GOMOD_BIN" ]] || { echo "ERRO: bootstrap de cyclonedx-gomod não produziu executável." >&2; exit 69; }
}

resolve_go
HOST_GOOS="$("$GO_BIN" env GOOS)"
HOST_GOARCH="$("$GO_BIN" env GOARCH)"
resolve_cyclonedx

for required in LICENSE NOTICE THIRD_PARTY_NOTICES.md; do
  [[ -f "$required" ]] || { echo "ERRO: arquivo legal obrigatório ausente: $required" >&2; exit 4; }
done
[[ -f "$NODE_VERSION_FILE" ]] || { echo "ERRO: .node-version ausente" >&2; exit 4; }
[[ -f "$FRONTEND_DIR/package-lock.json" ]] || { echo "ERRO: frontend/package-lock.json ausente; release exige dependências npm travadas" >&2; exit 4; }
for cmd in node npm; do command -v "$cmd" >/dev/null 2>&1 || { echo "ERRO: $cmd ausente; frontend faz parte da release" >&2; exit 69; }; done
required_node="$(tr -d '[:space:]' < "$NODE_VERSION_FILE")"
actual_node="$(node -p 'process.version.slice(1)')"
[[ "$actual_node" == "$required_node" ]] || { echo "ERRO: Node $actual_node detectado; release exige $required_node" >&2; exit 69; }

echo "Construindo frontend com Node $actual_node e npm $(npm --version)..."
(
  cd "$FRONTEND_DIR"
  npm ci --no-audit --no-fund
  npm run typecheck
  npm test
  npm run build
)
[[ -f "$FRONTEND_DIR/dist/index.html" ]] || { echo "ERRO: frontend/dist/index.html não foi gerado" >&2; exit 4; }
find "$FRONTEND_DIR/dist/assets" -maxdepth 1 -type f -name 'index-*.js' -print -quit | grep -q . || { echo "ERRO: bundle frontend hashed ausente" >&2; exit 4; }

rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"
for arch in $ARCHES; do
  pkg="rc-gateway_${VERSION}_linux_${arch}"
  stage="$DIST_DIR/$pkg"
  mkdir -p "$stage/bin" "$stage/systemd" "$stage/configs/monitor" "$stage/scripts" "$stage/docs" "$stage/controllers" "$stage/frontend"

  gateway_ldflags="-s -w -X main.version=$VERSION -X main.commit=$COMMIT -X main.buildDate=$BUILD_DATE"
  monitor_ldflags="-s -w -X main.version=$VERSION"
  admin_ldflags="-s -w -X main.version=$VERSION"
  CGO_ENABLED=0 GOOS=linux GOARCH="$arch" "$GO_BIN" build -trimpath -buildvcs=false -ldflags "$gateway_ldflags" -o "$stage/bin/rc-gateway" ./cmd/rc-gateway
  CGO_ENABLED=0 GOOS=linux GOARCH="$arch" "$GO_BIN" build -trimpath -buildvcs=false -ldflags "$monitor_ldflags" -o "$stage/bin/rc-monitor" ./cmd/rc-monitor
  CGO_ENABLED=0 GOOS=linux GOARCH="$arch" "$GO_BIN" build -trimpath -buildvcs=false -ldflags "$admin_ldflags" -o "$stage/bin/rc-admin" ./cmd/rc-admin

  cp systemd/rc-gateway.service systemd/rc-monitor.service systemd/rc-admin.service "$stage/systemd/"
  cp configs/*.json "$stage/configs/"
  cp configs/monitor/*.json "$stage/configs/monitor/"
  cp controllers/DRAFT_PROFILES.json controllers/README.md controllers/REFERENCE_CATALOG.md "$stage/controllers/"
  cp -R controllers/rc-simulator "$stage/controllers/"
  cp -R "$FRONTEND_DIR/dist/." "$stage/frontend/"

  cp scripts/install-release.sh scripts/install-scada-stack.sh scripts/install-rc-monitor.sh scripts/install-rc-admin.sh scripts/install-rc-frontend.sh scripts/install-rc-lab-stack.sh scripts/configure-rapid-web-api.sh scripts/rc-frontend-acceptance.sh scripts/rc-admin-acceptance.sh scripts/rollback-release.sh scripts/probe-usb-hid.sh scripts/collect-diagnostics.sh scripts/vm-acceptance.sh scripts/run-soak.sh scripts/rapid-scada-acceptance.sh scripts/rapid-scada-production-acceptance.sh "$stage/scripts/"
  cp docs/RUNBOOK.md docs/USB_HID_COMAP.md docs/COMPATIBILITY_MATRIX.md docs/PRODUCTION_MATRIX.md docs/VM_ACCEPTANCE.md docs/THREAT_MODEL.md docs/PROFESSIONALIZATION_PLAN.md docs/CONFIGURATION_COMPATIBILITY.md docs/RAPID_SCADA_INTEGRATION.md docs/GENERATOR_SCADA_PRODUCTION_READINESS.md docs/SCADA_STACK_INSTALLER.md docs/RC_MONITOR_OPERATIONS.md docs/RAPID_SCADA_MONITOR_BINDING.md "$stage/docs/"
  cp README.md SECURITY.md SUPPORT.md CHANGELOG.md LICENSE NOTICE THIRD_PARTY_NOTICES.md "$stage/"
  chmod 0755 "$stage/bin/rc-gateway" "$stage/bin/rc-monitor" "$stage/bin/rc-admin" "$stage/scripts/"*.sh

  [[ -f "$stage/frontend/index.html" ]] || { echo "ERRO: frontend não entrou na release $arch" >&2; exit 4; }
  find "$stage/frontend/assets" -maxdepth 1 -type f -name 'index-*.js' -print -quit | grep -q . || { echo "ERRO: assets frontend ausentes na release $arch" >&2; exit 4; }

  if [[ "$HOST_GOOS" == "linux" && "$HOST_GOARCH" == "$arch" ]]; then
    "$stage/bin/rc-monitor" --check-config --config "$stage/configs/monitor/rc-monitor.fake.json"
    "$stage/bin/rc-monitor" --check-config --config "$stage/configs/monitor/rc-monitor.synthetic.json"
    "$stage/scripts/install-rc-monitor.sh" --dry-run "$stage/configs/monitor/rc-monitor.fake.json"
    "$stage/scripts/install-rc-monitor.sh" --help >/dev/null
  fi

  printf '%s\n' "$VERSION" > "$stage/VERSION"
  {
    printf 'product=rc-gateway\n'
    printf 'component=rc-monitor\n'
    printf 'component=rc-monitor-frontend\n'
    printf 'component=rc-admin\n'
    printf 'license=Proprietary-All-Rights-Reserved\n'
    printf 'version=%s\n' "$VERSION"
    printf 'commit=%s\n' "$COMMIT"
    printf 'buildDate=%s\n' "$BUILD_DATE"
    printf 'node=%s\n' "$required_node"
    printf 'go=%s\n' "$("$GO_BIN" version)"
    printf 'goos=linux\n'
    printf 'goarch=%s\n' "$arch"
  } > "$stage/MANIFEST"

  if [[ -n "$CYCLONEDX_GOMOD_BIN" && -x "$CYCLONEDX_GOMOD_BIN" ]]; then
    GOOS=linux GOARCH="$arch" CGO_ENABLED=0 "$CYCLONEDX_GOMOD_BIN" app -json -noserial -notimestamp -output "$stage/sbom.cdx.json" -main cmd/rc-gateway "$ROOT_DIR"
  elif [[ "$REQUIRE_SBOM" == "1" ]]; then
    echo "ERRO: cyclonedx-gomod é obrigatório para esta build." >&2
    exit 3
  else
    echo "AVISO: SBOM não gerado." >&2
  fi

  archive="$DIST_DIR/$pkg.tar.gz"
  (
    cd "$DIST_DIR"
    tar --sort=name --mtime="@$SOURCE_DATE_EPOCH" --owner=0 --group=0 --numeric-owner -cf - "$pkg" | gzip -n > "$archive"
    sha256sum "$(basename "$archive")" > "$(basename "$archive").sha256"
  )
  rm -rf "$stage"
  echo "release criada: $archive"
done

# Standalone deployment kit files. Rapid SCADA itself remains a third-party package
# and is intentionally not embedded in the proprietary Gateway artifact.
cp scripts/install-scada-stack.sh "$DIST_DIR/install-scada-stack.sh"
cp scripts/install-rc-lab-stack.sh "$DIST_DIR/install-rc-lab-stack.sh"
cp configs/scada-stack.safe.example.json "$DIST_DIR/rc-gateway.safe.json"
cp docs/SCADA_STACK_INSTALLER.md "$DIST_DIR/SCADA_STACK_INSTALLER.md"
chmod 0755 "$DIST_DIR/install-scada-stack.sh" "$DIST_DIR/install-rc-lab-stack.sh"
