#!/usr/bin/env bash
set -Eeuo pipefail

GATEWAY_REPO="${RC_SCADA_GATEWAY_REPO:-paulohspred/Gateway}"
GATEWAY_REF="${RC_SCADA_GATEWAY_REF:-8f8d2560832be534ba1f9c5d58d3c1afacba8189}"
RAPID_VERSION="${RC_SCADA_RAPID_VERSION:-6.4.7}"
RAPID_URL="${RC_SCADA_RAPID_URL:-https://rapidscada.org/download/rapidscada_${RAPID_VERSION}_linux_en.zip}"
# Frozen checksum observed for the official Rapid SCADA 6.4.7 Linux package used in this acceptance cycle.
DEFAULT_RAPID_SHA256="48e8c8c33b8380fddc7c6012d6856123cb6328a1645e784e97a29aa525311180"
RAPID_SHA256_EXPECTED="${RC_SCADA_RAPID_SHA256:-$DEFAULT_RAPID_SHA256}"
GATEWAY_CONFIG_URL="${RC_SCADA_GATEWAY_CONFIG_URL:-}"
KEEP_WORK="${RC_SCADA_KEEP_BOOTSTRAP_WORKDIR:-0}"
DRY_RUN=0

usage() {
  cat >&2 <<'USAGE'
Uso:
  sudo bash bootstrap-scada-stack-v2.sh [--dry-run]

Instala automaticamente RC Gateway + Rapid SCADA v6.4.7 em Ubuntu Server 24.04.
O Gateway é obtido por Git no commit pinado para preservar metadata necessária à geração do SBOM.
USAGE
}

log() { printf '[rc-scada-bootstrap] %s\n' "$*"; }
warn() { printf '[rc-scada-bootstrap] AVISO: %s\n' "$*" >&2; }
die() { printf '[rc-scada-bootstrap] ERRO: %s\n' "$*" >&2; exit "${2:-1}"; }

while (($#)); do
  case "$1" in
    --dry-run) DRY_RUN=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "opção desconhecida: $1" 64 ;;
  esac
done

[[ "$KEEP_WORK" == "0" || "$KEEP_WORK" == "1" ]] || die "RC_SCADA_KEEP_BOOTSTRAP_WORKDIR deve ser 0 ou 1" 64
[[ "$GATEWAY_REF" =~ ^[0-9a-fA-F]{40}$ ]] || die "RC_SCADA_GATEWAY_REF deve ser um commit SHA completo" 64
[[ "$RAPID_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "RC_SCADA_RAPID_VERSION inválida: $RAPID_VERSION" 64
[[ "$RAPID_URL" == https://* ]] || die "RC_SCADA_RAPID_URL deve usar HTTPS" 64
[[ "$RAPID_SHA256_EXPECTED" =~ ^[0-9a-fA-F]{64}$ ]] || die "RC_SCADA_RAPID_SHA256 inválido" 64
if [[ -n "$GATEWAY_CONFIG_URL" && "$GATEWAY_CONFIG_URL" != https://* ]]; then
  die "RC_SCADA_GATEWAY_CONFIG_URL deve usar HTTPS" 64
fi

[[ $EUID -eq 0 ]] || die "execute com sudo/root" 77
[[ -r /etc/os-release ]] || die "/etc/os-release ausente" 69
# shellcheck disable=SC1091
source /etc/os-release
[[ "${ID:-}" == "ubuntu" && "${VERSION_ID:-}" == "24.04" ]] || die "baseline suportado: Ubuntu Server 24.04. Detectado: ${PRETTY_NAME:-desconhecido}" 65

export DEBIAN_FRONTEND=noninteractive
log "preparando dependências de bootstrap"
apt-get update
apt-get install -y --no-install-recommends ca-certificates curl git tar gzip unzip dpkg golang-go

for cmd in curl git tar gzip sha256sum unzip dpkg-deb go; do
  command -v "$cmd" >/dev/null 2>&1 || die "comando obrigatório ausente após bootstrap: $cmd" 69
done

case "$(dpkg --print-architecture)" in
  amd64) ARCH=amd64 ;;
  arm64) ARCH=arm64 ;;
  *) die "arquitetura não suportada: $(dpkg --print-architecture)" 65 ;;
esac

WORK_DIR="$(mktemp -d /tmp/rc-scada-bootstrap.XXXXXX)"
cleanup() {
  if [[ "$KEEP_WORK" == "1" ]]; then
    warn "diretório temporário preservado: $WORK_DIR"
  else
    rm -rf "$WORK_DIR"
  fi
}
trap cleanup EXIT
KIT_DIR="$WORK_DIR/kit"
GATEWAY_SOURCE_DIR="$WORK_DIR/source"
RAPID_DOWNLOAD="$WORK_DIR/rapidscada_${RAPID_VERSION}_linux_en.zip"
RAPID_ARCHIVE="$KIT_DIR/rapidscada_${RAPID_VERSION}_linux_en.zip"
CUSTOM_CONFIG_DOWNLOAD="$WORK_DIR/rc-gateway.custom.json"
mkdir -p "$KIT_DIR"

curl_get() {
  local url="$1" dest="$2"
  curl --fail --show-error --location --progress-bar \
    --proto '=https' --proto-redir '=https' --tlsv1.2 \
    --retry 5 --retry-delay 2 --retry-all-errors \
    --connect-timeout 20 --max-time 900 \
    --output "$dest" "$url"
  [[ -s "$dest" ]] || die "download vazio: $url" 3
}

GATEWAY_GIT_URL="https://github.com/${GATEWAY_REPO}.git"

log "clonando Gateway com metadata Git"
git -c advice.detachedHead=false clone --quiet "$GATEWAY_GIT_URL" "$GATEWAY_SOURCE_DIR"
git -C "$GATEWAY_SOURCE_DIR" checkout --quiet --detach "$GATEWAY_REF"
ACTUAL_GATEWAY_REF="$(git -C "$GATEWAY_SOURCE_DIR" rev-parse HEAD)"
[[ "${ACTUAL_GATEWAY_REF,,}" == "${GATEWAY_REF,,}" ]] || die "checkout Gateway não corresponde ao commit pinado" 4
log "Gateway fixado em $ACTUAL_GATEWAY_REF"

for required in scripts/build-release.sh scripts/install-scada-stack.sh configs/scada-stack.safe.example.json go.mod LICENSE NOTICE THIRD_PARTY_NOTICES.md; do
  [[ -e "$GATEWAY_SOURCE_DIR/$required" ]] || die "fonte Gateway incompleta: $required" 4
done

PROJECT_GO_VERSION="$(awk '$1 == "go" {print $2; exit}' "$GATEWAY_SOURCE_DIR/go.mod")"
[[ "$PROJECT_GO_VERSION" =~ ^[0-9]+\.[0-9]+(\.[0-9]+)?$ ]] || die "versão Go inválida em go.mod: $PROJECT_GO_VERSION" 4
PROJECT_GOTOOLCHAIN="go${PROJECT_GO_VERSION}+auto"
log "preparando toolchain Go ${PROJECT_GO_VERSION} exigida pelo Gateway"
PROJECT_GOROOT="$(GOTOOLCHAIN="$PROJECT_GOTOOLCHAIN" go env GOROOT)"
[[ -x "$PROJECT_GOROOT/bin/go" ]] || die "toolchain Go ${PROJECT_GO_VERSION} não foi resolvida" 69
PROJECT_GO_BIN="$PROJECT_GOROOT/bin"
log "toolchain ativa: $($PROJECT_GO_BIN/go version)"

# Inputs externos são baixados fora de KIT_DIR porque build-release.sh recria
# integralmente DIST_DIR. Depois da build eles são copiados para o kit final.
log "baixando Rapid SCADA ${RAPID_VERSION} do fornecedor oficial"
curl_get "$RAPID_URL" "$RAPID_DOWNLOAD"
RAPID_DOWNLOADED_SHA="$(sha256sum "$RAPID_DOWNLOAD" | awk '{print $1}')"
[[ "${RAPID_DOWNLOADED_SHA,,}" == "${RAPID_SHA256_EXPECTED,,}" ]] || die "SHA256 do Rapid SCADA não confere. esperado=$RAPID_SHA256_EXPECTED obtido=$RAPID_DOWNLOADED_SHA" 3
log "Rapid SCADA SHA256 conferido: $RAPID_DOWNLOADED_SHA"

if [[ -n "$GATEWAY_CONFIG_URL" ]]; then
  log "baixando configuração personalizada do Gateway"
  curl_get "$GATEWAY_CONFIG_URL" "$CUSTOM_CONFIG_DOWNLOAD"
fi

log "gerando artifact Gateway local com SBOM"
TOOL_BIN="$WORK_DIR/gobin"
mkdir -p "$TOOL_BIN"
PATH="$PROJECT_GO_BIN:$PATH" \
  GOROOT="$PROJECT_GOROOT" \
  GOBIN="$TOOL_BIN" \
  GOTOOLCHAIN="$PROJECT_GOTOOLCHAIN" \
  "$PROJECT_GO_BIN/go" install github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@07257d5b9cbd2a3d4338a880c0ca50081e1ac445

(
  cd "$GATEWAY_SOURCE_DIR"
  VERSION="vm-${GATEWAY_REF:0:12}"
  PATH="$TOOL_BIN:$PROJECT_GO_BIN:$PATH" \
    GOROOT="$PROJECT_GOROOT" \
    GOTOOLCHAIN="$PROJECT_GOTOOLCHAIN" \
    COMMIT="$GATEWAY_REF" \
    ARCHES="$ARCH" \
    REQUIRE_SBOM=1 \
    DIST_DIR="$KIT_DIR" \
    bash scripts/build-release.sh "$VERSION"
)

[[ -x "$KIT_DIR/install-scada-stack.sh" ]] || die "build não gerou install-scada-stack.sh" 4
mapfile -t built_gateway_archives < <(find "$KIT_DIR" -maxdepth 1 -type f -name "rc-gateway_*_linux_${ARCH}.tar.gz" -print)
[[ ${#built_gateway_archives[@]} -eq 1 ]] || die "build deveria gerar exatamente um archive Gateway para $ARCH" 4
[[ -f "${built_gateway_archives[0]}.sha256" ]] || die "build não gerou checksum do Gateway" 4

log "montando kit final Gateway + Rapid SCADA"
cp "$RAPID_DOWNLOAD" "$RAPID_ARCHIVE"
[[ "$(sha256sum "$RAPID_ARCHIVE" | awk '{print $1}')" == "$RAPID_DOWNLOADED_SHA" ]] || die "Rapid SCADA foi alterado ao montar o kit" 3
if [[ -n "$GATEWAY_CONFIG_URL" ]]; then
  cp "$CUSTOM_CONFIG_DOWNLOAD" "$KIT_DIR/rc-gateway.json"
fi

log "validando kit completo antes de instalar"
install_args=(--dir "$KIT_DIR" --rapid-version "$RAPID_VERSION")
if [[ -n "$GATEWAY_CONFIG_URL" ]]; then
  install_args+=(--gateway-config "$KIT_DIR/rc-gateway.json")
fi
RC_SCADA_RAPID_SHA256="$RAPID_DOWNLOADED_SHA" bash "$KIT_DIR/install-scada-stack.sh" --dry-run "${install_args[@]}"

if [[ $DRY_RUN -eq 1 ]]; then
  log "DRY-RUN COMPLETO OK"
  exit 0
fi

log "instalando RC Gateway + Rapid SCADA"
RC_SCADA_RAPID_SHA256="$RAPID_DOWNLOADED_SHA" bash "$KIT_DIR/install-scada-stack.sh" "${install_args[@]}"

log "INSTALAÇÃO AUTOMÁTICA CONCLUÍDA"
log "Gateway readiness: http://127.0.0.1:18080/readyz"
log "Rapid SCADA Webstation: http://127.0.0.1/ (loopback por segurança)"
log "Estado: /var/lib/rc-scada-stack/install-state.env"
