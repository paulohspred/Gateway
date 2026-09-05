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
rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"
for arch in $ARCHES; do
  pkg="rc-gateway-umbrella_${VERSION}_linux_${arch}"
  stage="$DIST_DIR/$pkg"
  mkdir -p "$stage/bin" "$stage/systemd" "$stage/configs" "$stage/scripts"
  ldflags="-s -w -X main.version=$VERSION -X main.commit=$COMMIT -X main.buildDate=$BUILD_DATE"
  CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -buildvcs=false -ldflags "$ldflags" -o "$stage/bin/rc-gateway" ./cmd/rc-gateway
  cp systemd/rc-gateway-umbrella.service "$stage/systemd/"
  cp configs/*.json "$stage/configs/"
  cp scripts/install-release.sh scripts/rollback-release.sh "$stage/scripts/"
  chmod 0755 "$stage/bin/rc-gateway" "$stage/scripts/"*.sh
  printf '%s\n' "$VERSION" > "$stage/VERSION"
  {
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
