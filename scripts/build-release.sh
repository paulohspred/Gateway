#!/usr/bin/env bash
set -Eeuo pipefail
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
VERSION="${1:-${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || git rev-parse --short HEAD)}}"
VERSION="${VERSION//\//-}"
[[ "$VERSION" =~ ^[A-Za-z0-9._+-]+$ ]] || { echo "ERRO: versão inválida: $VERSION" >&2; exit 2; }
COMMIT="${COMMIT:-$(git rev-parse HEAD 2>/dev/null || printf unknown)}"
SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH:-$(git show -s --format=%ct HEAD 2>/dev/null || date +%s)}"
BUILD_DATE="${BUILD_DATE:-$(date -u -d "@$SOURCE_DATE_EPOCH" +%Y-%m-%dT%H:%M:%SZ)}"
ARCHES="${ARCHES:-amd64 arm64}"
DIST_DIR="${DIST_DIR:-$ROOT_DIR/dist}"
REQUIRE_SBOM="${REQUIRE_SBOM:-0}"

for required in LICENSE NOTICE THIRD_PARTY_NOTICES.md; do
  [[ -f "$required" ]] || { echo "ERRO: arquivo legal obrigatório ausente: $required" >&2; exit 4; }
done

rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"
for arch in $ARCHES; do
  pkg="rc-gateway_${VERSION}_linux_${arch}"
  stage="$DIST_DIR/$pkg"
  mkdir -p "$stage/bin" "$stage/systemd" "$stage/configs/monitor" "$stage/scripts" "$stage/docs" "$stage/controllers"

  gateway_ldflags="-s -w -X main.version=$VERSION -X main.commit=$COMMIT -X main.buildDate=$BUILD_DATE"
  monitor_ldflags="-s -w -X main.version=$VERSION"
  CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -buildvcs=false -ldflags "$gateway_ldflags" -o "$stage/bin/rc-gateway" ./cmd/rc-gateway
  CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -buildvcs=false -ldflags "$monitor_ldflags" -o "$stage/bin/rc-monitor" ./cmd/rc-monitor

  cp systemd/rc-gateway.service systemd/rc-monitor.service "$stage/systemd/"
  cp configs/*.json "$stage/configs/"
  cp configs/monitor/*.json "$stage/configs/monitor/"
  cp controllers/DRAFT_PROFILES.json controllers/README.md controllers/REFERENCE_CATALOG.md "$stage/controllers/"
  cp -R controllers/rc-simulator "$stage/controllers/"

  cp scripts/install-release.sh scripts/install-scada-stack.sh scripts/install-rc-monitor.sh scripts/rollback-release.sh scripts/probe-usb-hid.sh scripts/collect-diagnostics.sh scripts/vm-acceptance.sh scripts/run-soak.sh scripts/rapid-scada-acceptance.sh scripts/rapid-scada-production-acceptance.sh "$stage/scripts/"
  cp docs/RUNBOOK.md docs/USB_HID_COMAP.md docs/COMPATIBILITY_MATRIX.md docs/PRODUCTION_MATRIX.md docs/VM_ACCEPTANCE.md docs/THREAT_MODEL.md docs/PROFESSIONALIZATION_PLAN.md docs/CONFIGURATION_COMPATIBILITY.md docs/RAPID_SCADA_INTEGRATION.md docs/GENERATOR_SCADA_PRODUCTION_READINESS.md docs/SCADA_STACK_INSTALLER.md docs/RC_MONITOR_OPERATIONS.md docs/RAPID_SCADA_MONITOR_BINDING.md "$stage/docs/"
  cp README.md SECURITY.md SUPPORT.md CHANGELOG.md LICENSE NOTICE THIRD_PARTY_NOTICES.md "$stage/"
  chmod 0755 "$stage/bin/rc-gateway" "$stage/bin/rc-monitor" "$stage/scripts/"*.sh

  printf '%s\n' "$VERSION" > "$stage/VERSION"
  {
    printf 'product=rc-gateway\n'
    printf 'component=rc-monitor\n'
    printf 'license=Proprietary-All-Rights-Reserved\n'
    printf 'version=%s\n' "$VERSION"
    printf 'commit=%s\n' "$COMMIT"
    printf 'buildDate=%s\n' "$BUILD_DATE"
    printf 'goos=linux\n'
    printf 'goarch=%s\n' "$arch"
  } > "$stage/MANIFEST"

  if command -v cyclonedx-gomod >/dev/null 2>&1; then
    GOOS=linux GOARCH="$arch" CGO_ENABLED=0 cyclonedx-gomod app -json -noserial -notimestamp -output "$stage/sbom.cdx.json" -main cmd/rc-gateway "$ROOT_DIR"
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
cp configs/scada-stack.safe.example.json "$DIST_DIR/rc-gateway.safe.json"
cp docs/SCADA_STACK_INSTALLER.md "$DIST_DIR/SCADA_STACK_INSTALLER.md"
chmod 0755 "$DIST_DIR/install-scada-stack.sh"
