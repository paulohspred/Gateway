#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SOURCE_DIR="$SCRIPT_DIR"
DRY_RUN=0
OFFLINE=0
ALLOW_UPGRADE=0
GATEWAY_CONFIG=""
EXPECTED_RAPID_VERSION="${RC_SCADA_RAPID_VERSION:-6.4.7}"
ALLOW_UNSUPPORTED_OS="${RC_SCADA_ALLOW_UNSUPPORTED_OS:-0}"
RAPID_SHA256_EXPECTED="${RC_SCADA_RAPID_SHA256:-}"
STATE_DIR="/var/lib/rc-scada-stack"
WEB_OVERRIDE="/etc/systemd/system/scadaweb6.service.d/10-rc-scada-loopback.conf"
NGINX_SITE="/etc/nginx/sites-available/rc-scada"
NGINX_LINK="/etc/nginx/sites-enabled/rc-scada"

usage() {
  local code="${1:-64}"
  cat >&2 <<'USAGE'
Uso:
  sudo bash install-scada-stack.sh [opções]

Opções:
  --dir DIR                 Diretório contendo os pacotes (padrão: diretório do script)
  --gateway-config FILE     Config JSON do Gateway. Se omitida, usa rc-gateway.json,
                            rc-gateway.safe.json ou gera configuração segura sem tunnels.
  --rapid-version VERSION   Versão Rapid SCADA esperada (padrão: 6.4.7)
  --offline                 Não usa apt para baixar dependências; exige pré-requisitos locais.
  --upgrade                 Permite atualização quando Gateway/Rapid já existem.
  --dry-run                 Valida pacote, checksum e configuração sem instalar.
  -h, --help                Mostra esta ajuda.

Diretório mínimo:
  install-scada-stack.sh
  rc-gateway_<versao>_linux_<amd64|arm64>.tar.gz
  rc-gateway_<versao>_linux_<amd64|arm64>.tar.gz.sha256
  rapidscada_<versao>_all.deb

Também é aceito o ZIP oficial Linux do Rapid SCADA, desde que unzip esteja disponível
(ou possa ser instalado via apt). Um rc-gateway.json é opcional; sem ele o Gateway
é instalado em estado seguro, sem tunnels de campo.
USAGE
  exit "$code"
}

log() { printf '[rc-scada-stack] %s\n' "$*"; }
warn() { printf '[rc-scada-stack] AVISO: %s\n' "$*" >&2; }
die() { printf '[rc-scada-stack] ERRO: %s\n' "$*" >&2; exit "${2:-1}"; }

while (($#)); do
  case "$1" in
    --dir)
      [[ $# -ge 2 ]] || usage
      SOURCE_DIR="$2"
      shift 2
      ;;
    --gateway-config)
      [[ $# -ge 2 ]] || usage
      GATEWAY_CONFIG="$2"
      shift 2
      ;;
    --rapid-version)
      [[ $# -ge 2 ]] || usage
      EXPECTED_RAPID_VERSION="$2"
      shift 2
      ;;
    --offline) OFFLINE=1; shift ;;
    --upgrade) ALLOW_UPGRADE=1; shift ;;
    --dry-run) DRY_RUN=1; shift ;;
    -h|--help) usage 0 ;;
    *) die "opção desconhecida: $1" 64 ;;
  esac
done

[[ "$EXPECTED_RAPID_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.+~-][A-Za-z0-9.+:~-]+)?$ ]] || die "versão Rapid SCADA inválida: $EXPECTED_RAPID_VERSION" 64
[[ "$ALLOW_UNSUPPORTED_OS" == "0" || "$ALLOW_UNSUPPORTED_OS" == "1" ]] || die "RC_SCADA_ALLOW_UNSUPPORTED_OS deve ser 0 ou 1" 64
SOURCE_DIR="$(realpath "$SOURCE_DIR")"
[[ -d "$SOURCE_DIR" ]] || die "diretório inexistente: $SOURCE_DIR" 2

for cmd in realpath sha256sum tar awk grep find mktemp dpkg-deb; do
  command -v "$cmd" >/dev/null 2>&1 || die "comando obrigatório ausente: $cmd" 69
done

arch=""
if command -v dpkg >/dev/null 2>&1; then
  arch="$(dpkg --print-architecture 2>/dev/null || true)"
fi
if [[ -z "$arch" ]]; then
  case "$(uname -m)" in
    x86_64) arch=amd64 ;;
    aarch64|arm64) arch=arm64 ;;
    *) die "arquitetura não suportada: $(uname -m)" 65 ;;
  esac
fi
case "$arch" in amd64|arm64) ;; *) die "arquitetura não suportada pelo release: $arch" 65 ;; esac

mapfile -t gateway_archives < <(find "$SOURCE_DIR" -maxdepth 1 -type f -name "rc-gateway_*_linux_${arch}.tar.gz" -print | sort)
[[ ${#gateway_archives[@]} -eq 1 ]] || die "esperado exatamente 1 archive Gateway para $arch em $SOURCE_DIR; encontrados ${#gateway_archives[@]}" 2
GATEWAY_ARCHIVE="${gateway_archives[0]}"
GATEWAY_CHECKSUM="$GATEWAY_ARCHIVE.sha256"
[[ -f "$GATEWAY_CHECKSUM" ]] || die "checksum ausente: $GATEWAY_CHECKSUM" 2

expected_gateway_sha="$(awk 'NF {print $1; exit}' "$GATEWAY_CHECKSUM")"
actual_gateway_sha="$(sha256sum "$GATEWAY_ARCHIVE" | awk '{print $1}')"
[[ "$expected_gateway_sha" =~ ^[0-9a-fA-F]{64}$ ]] || die "checksum Gateway inválido" 3
[[ "${expected_gateway_sha,,}" == "${actual_gateway_sha,,}" ]] || die "SHA256 do Gateway não confere" 3

if tar -tzf "$GATEWAY_ARCHIVE" | grep -Eq '(^/|(^|/)\.\.(/|$))'; then
  die "archive Gateway contém caminho inseguro" 4
fi
unsafe_gateway_entry="$(tar -tvzf "$GATEWAY_ARCHIVE" | awk 'substr($1,1,1)!="-" && substr($1,1,1)!="d" {print; exit}')"
[[ -z "$unsafe_gateway_entry" ]] || die "archive Gateway contém tipo de entrada não permitido: $unsafe_gateway_entry" 4
mapfile -t embedded_installers < <(tar -tzf "$GATEWAY_ARCHIVE" | grep -E '/scripts/install-release\.sh$' || true)
[[ ${#embedded_installers[@]} -eq 1 ]] || die "install-release.sh não encontrado unicamente no archive Gateway" 4
mapfile -t gateway_versions < <(tar -tzf "$GATEWAY_ARCHIVE" | grep -E '/VERSION$' || true)
[[ ${#gateway_versions[@]} -eq 1 ]] || die "VERSION não encontrado unicamente no archive Gateway" 4
GATEWAY_VERSION="$(tar -xOzf "$GATEWAY_ARCHIVE" "${gateway_versions[0]}" | tr -d '\r\n')"
[[ "$GATEWAY_VERSION" =~ ^[A-Za-z0-9._+-]+$ ]] || die "VERSION do Gateway inválida" 4

tmp="$(mktemp -d)"
cleanup() { rm -rf "$tmp"; }
trap cleanup EXIT
EMBEDDED_INSTALLER="$tmp/install-release.sh"
tar -xOzf "$GATEWAY_ARCHIVE" "${embedded_installers[0]}" > "$EMBEDDED_INSTALLER"
chmod 0700 "$EMBEDDED_INSTALLER"
bash -n "$EMBEDDED_INSTALLER"

resolve_gateway_config() {
  if [[ -n "$GATEWAY_CONFIG" ]]; then
    GATEWAY_CONFIG="$(realpath "$GATEWAY_CONFIG")"
    [[ -f "$GATEWAY_CONFIG" ]] || die "configuração Gateway ausente: $GATEWAY_CONFIG" 2
    return
  fi
  if [[ -f "$SOURCE_DIR/rc-gateway.json" ]]; then
    GATEWAY_CONFIG="$SOURCE_DIR/rc-gateway.json"
    return
  fi
  if [[ -f "$SOURCE_DIR/rc-gateway.safe.json" ]]; then
    GATEWAY_CONFIG="$SOURCE_DIR/rc-gateway.safe.json"
    return
  fi
  GATEWAY_CONFIG="$tmp/rc-gateway.safe.json"
  cat > "$GATEWAY_CONFIG" <<'JSON'
{
  "schema": 3,
  "nodeId": "rc-gateway-scada-stack",
  "admin": {"bind": "127.0.0.1:18080"},
  "security": {
    "requireAllowlist": true,
    "commandPlaneEnabled": false
  },
  "limits": {"maxActivePairs": 1024},
  "tunnels": []
}
JSON
  warn "rc-gateway.json não encontrado; será usada configuração segura sem tunnels de campo."
}
resolve_gateway_config

# Defense in depth: the embedded installer revalidates the archive and executes the
# exact Gateway binary against the candidate config before any mutation.
bash "$EMBEDDED_INSTALLER" --dry-run "$GATEWAY_ARCHIVE" "$GATEWAY_CHECKSUM" "$GATEWAY_CONFIG" >/dev/null

mapfile -t rapid_debs < <(find "$SOURCE_DIR" -maxdepth 1 -type f -name 'rapidscada_*_all.deb' -print | sort)
mapfile -t rapid_zips < <(find "$SOURCE_DIR" -maxdepth 1 -type f -name 'rapidscada_*_linux*.zip' -print | sort)
if (( ${#rapid_debs[@]} + ${#rapid_zips[@]} != 1 )); then
  die "coloque exatamente 1 rapidscada_*_all.deb OU 1 ZIP Linux oficial do Rapid SCADA em $SOURCE_DIR" 2
fi
RAPID_SOURCE=""
RAPID_DEB=""
RAPID_SOURCE_KIND=""
if [[ ${#rapid_debs[@]} -eq 1 ]]; then
  RAPID_SOURCE="${rapid_debs[0]}"
  RAPID_DEB="$RAPID_SOURCE"
  RAPID_SOURCE_KIND="deb"
else
  RAPID_SOURCE="${rapid_zips[0]}"
  RAPID_SOURCE_KIND="zip"
fi

verify_optional_source_checksum() {
  local source="$1"
  local checksum="$source.sha256"
  if [[ -n "$RAPID_SHA256_EXPECTED" ]]; then
    [[ "$RAPID_SHA256_EXPECTED" =~ ^[0-9a-fA-F]{64}$ ]] || die "RC_SCADA_RAPID_SHA256 inválido" 64
    local actual
    actual="$(sha256sum "$source" | awk '{print $1}')"
    [[ "${actual,,}" == "${RAPID_SHA256_EXPECTED,,}" ]] || die "SHA256 do pacote Rapid SCADA não confere com RC_SCADA_RAPID_SHA256" 3
    return
  fi
  if [[ -f "$checksum" ]]; then
    local expected actual
    expected="$(awk 'NF {print $1; exit}' "$checksum")"
    actual="$(sha256sum "$source" | awk '{print $1}')"
    [[ "$expected" =~ ^[0-9a-fA-F]{64}$ ]] || die "checksum Rapid SCADA inválido: $checksum" 3
    [[ "${actual,,}" == "${expected,,}" ]] || die "SHA256 do pacote Rapid SCADA não confere" 3
  else
    warn "pacote Rapid SCADA não possui checksum local/pinado; registre RC_SCADA_RAPID_SHA256 no kit de produção."
  fi
}
verify_optional_source_checksum "$RAPID_SOURCE"

check_supported_os() {
  [[ -r /etc/os-release ]] || die "/etc/os-release ausente" 69
  # shellcheck disable=SC1091
  source /etc/os-release
  if [[ "${ID:-}" == "ubuntu" && "${VERSION_ID:-}" == "24.04" ]]; then
    return
  fi
  if [[ "$ALLOW_UNSUPPORTED_OS" == "1" ]]; then
    warn "SO fora do baseline Ubuntu 24.04: ${PRETTY_NAME:-desconhecido}. Execução permitida apenas por override."
    return
  fi
  die "baseline suportado pelo instalador: Ubuntu Server 24.04. Detectado: ${PRETTY_NAME:-desconhecido}. Para laboratório, RC_SCADA_ALLOW_UNSUPPORTED_OS=1." 65
}

extract_rapid_from_zip() {
  command -v unzip >/dev/null 2>&1 || die "unzip é obrigatório para pacote Rapid SCADA ZIP" 69
  mapfile -t rapid_entries < <(unzip -Z1 "$RAPID_SOURCE" | grep -E '(^|/)rapidscada_[^/]+_all\.deb$' || true)
  [[ ${#rapid_entries[@]} -eq 1 ]] || die "ZIP Rapid SCADA deve conter exatamente um rapidscada_*_all.deb" 4
  local entry="${rapid_entries[0]}"
  [[ ! "$entry" =~ ^/ && ! "$entry" =~ (^|/)\.\.(/|$) ]] || die "ZIP Rapid SCADA contém caminho inseguro para o .deb" 4
  RAPID_DEB="$tmp/rapidscada.deb"
  unzip -p "$RAPID_SOURCE" "$entry" > "$RAPID_DEB"
  [[ -s "$RAPID_DEB" ]] || die "falha ao extrair .deb do ZIP Rapid SCADA" 4
}

validate_rapid_deb() {
  [[ -f "$RAPID_DEB" ]] || die "pacote Rapid SCADA ausente" 2
  local package version package_arch
  package="$(dpkg-deb -f "$RAPID_DEB" Package 2>/dev/null || true)"
  version="$(dpkg-deb -f "$RAPID_DEB" Version 2>/dev/null || true)"
  package_arch="$(dpkg-deb -f "$RAPID_DEB" Architecture 2>/dev/null || true)"
  [[ "$package" == "rapidscada" ]] || die "pacote .deb não é Rapid SCADA (Package=$package)" 4
  [[ "$version" == "$EXPECTED_RAPID_VERSION" || "$version" == "$EXPECTED_RAPID_VERSION"-* || "$version" == "$EXPECTED_RAPID_VERSION"+* ]] || die "versão Rapid SCADA $version difere do baseline exigido $EXPECTED_RAPID_VERSION" 4
  [[ "$package_arch" == "all" || "$package_arch" == "$arch" ]] || die "arquitetura Rapid SCADA incompatível: $package_arch (host=$arch)" 4
  RAPID_VERSION="$version"
  RAPID_DEB_SHA256="$(sha256sum "$RAPID_DEB" | awk '{print $1}')"
}

if [[ "$RAPID_SOURCE_KIND" == "zip" && $DRY_RUN -eq 1 && ! -x "$(command -v unzip || true)" ]]; then
  die "dry-run de ZIP exige unzip instalado; extraia o .deb ou instale unzip" 69
fi
if [[ "$RAPID_SOURCE_KIND" == "zip" ]]; then
  if command -v unzip >/dev/null 2>&1; then
    extract_rapid_from_zip
    validate_rapid_deb
  elif [[ $DRY_RUN -eq 1 || $OFFLINE -eq 1 ]]; then
    die "unzip ausente e não pode ser instalado neste modo" 69
  fi
else
  validate_rapid_deb
fi

if [[ $DRY_RUN -eq 1 ]]; then
  log "DRY-RUN OK"
  log "Gateway: version=$GATEWAY_VERSION arch=$arch sha256=$actual_gateway_sha"
  if [[ -n "${RAPID_VERSION:-}" ]]; then
    log "Rapid SCADA: version=$RAPID_VERSION source=$(basename "$RAPID_SOURCE") debSha256=$RAPID_DEB_SHA256"
  else
    log "Rapid SCADA ZIP localizado; metadata será validada após instalar unzip."
  fi
  log "Config Gateway: $GATEWAY_CONFIG"
  exit 0
fi

[[ $EUID -eq 0 ]] || die "instalação exige root (use sudo)" 77
check_supported_os
command -v systemctl >/dev/null 2>&1 || die "systemctl ausente" 69
[[ -d /run/systemd/system ]] || die "systemd não está ativo neste host" 69

if [[ $ALLOW_UPGRADE -eq 0 ]]; then
  if dpkg-query -W -f='${Status}' rapidscada 2>/dev/null | grep -q 'install ok installed'; then
    die "Rapid SCADA já está instalado. Use --upgrade somente após backup e janela autorizada." 5
  fi
  if [[ -e /opt/rc-gateway/current || -e /etc/rc-gateway.json ]]; then
    die "RC Gateway já está instalado. Use --upgrade somente após backup e janela autorizada." 5
  fi
fi

apt_install() {
  if [[ $OFFLINE -eq 1 ]]; then
    return 1
  fi
  DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends "$@"
}

if [[ $OFFLINE -eq 0 ]]; then
  command -v apt-get >/dev/null 2>&1 || die "apt-get ausente" 69
  log "Atualizando índice APT..."
  apt-get update
  log "Instalando pré-requisitos do stack..."
  apt_install ca-certificates curl nginx unzip || die "falha ao instalar pré-requisitos" 69
else
  for cmd in curl nginx unzip; do command -v "$cmd" >/dev/null 2>&1 || die "modo offline: comando ausente: $cmd" 69; done
fi

if [[ "$RAPID_SOURCE_KIND" == "zip" && -z "$RAPID_DEB" ]]; then
  extract_rapid_from_zip
  validate_rapid_deb
fi

has_aspnet8() {
  command -v dotnet >/dev/null 2>&1 && dotnet --list-runtimes 2>/dev/null | grep -Eq '^Microsoft\.AspNetCore\.App 8\.'
}
if ! has_aspnet8; then
  if [[ $OFFLINE -eq 1 ]]; then
    die "modo offline: ASP.NET Core Runtime 8.0.x não está instalado" 69
  fi
  log "Instalando ASP.NET Core Runtime 8.0..."
  if ! apt_install aspnetcore-runtime-8.0; then
    die "aspnetcore-runtime-8.0 não está disponível nos repositórios APT configurados. Instale o runtime 8.0 conforme a documentação oficial e execute novamente." 69
  fi
fi
has_aspnet8 || die "ASP.NET Core Runtime 8.0.x não foi detectado após instalação" 69

log "Instalando Rapid SCADA $RAPID_VERSION..."
if [[ $OFFLINE -eq 1 ]]; then
  dpkg -i "$RAPID_DEB"
else
  DEBIAN_FRONTEND=noninteractive apt-get install -y "$RAPID_DEB"
fi
[[ -d /opt/scada ]] || die "Rapid SCADA instalado, mas /opt/scada não existe" 6
for svc in scadaagent6.service scadaserver6.service scadacomm6.service scadaweb6.service; do
  systemctl cat "$svc" >/dev/null 2>&1 || die "unit Rapid SCADA ausente: $svc" 6
done

log "Aplicando exposição segura do Rapid SCADA Webstation..."
install -d -o root -g root -m 0755 "$(dirname "$WEB_OVERRIDE")"
cat > "$WEB_OVERRIDE" <<'UNIT'
[Service]
ExecStart=
ExecStart=/usr/bin/dotnet /opt/scada/ScadaWeb/ScadaWeb.dll --urls=http://127.0.0.1:10008
UNIT
chmod 0644 "$WEB_OVERRIDE"

# A VM limpa terá apenas o site default do Nginx. Não sobrescrevemos sites de terceiros.
if [[ -d /etc/nginx/sites-enabled ]]; then
  mapfile -t foreign_sites < <(find /etc/nginx/sites-enabled -mindepth 1 -maxdepth 1 \! -name default \! -name rc-scada -print)
  if [[ ${#foreign_sites[@]} -gt 0 && "${RC_SCADA_ALLOW_EXISTING_NGINX:-0}" != "1" ]]; then
    die "Nginx já possui sites não gerenciados (${foreign_sites[*]}). Revise manualmente ou use RC_SCADA_ALLOW_EXISTING_NGINX=1." 5
  fi
fi
rm -f /etc/nginx/sites-enabled/default
cat > "$NGINX_SITE" <<'NGINX'
server {
    listen 127.0.0.1:80 default_server;
    server_name _;

    client_max_body_size 32m;

    location / {
        proxy_pass http://127.0.0.1:10008;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
NGINX
chmod 0644 "$NGINX_SITE"
ln -sfn "$NGINX_SITE" "$NGINX_LINK"
nginx -t

log "Instalando RC Gateway $GATEWAY_VERSION..."
bash "$EMBEDDED_INSTALLER" "$GATEWAY_ARCHIVE" "$GATEWAY_CHECKSUM" "$GATEWAY_CONFIG"

log "Habilitando serviços Rapid SCADA e Nginx..."
systemctl daemon-reload
for svc in scadaagent6.service scadaserver6.service scadacomm6.service scadaweb6.service nginx.service; do
  systemctl enable "$svc" >/dev/null
  systemctl restart "$svc"
done

wait_service() {
  local svc="$1"
  local attempts="${2:-60}"
  for ((i=1; i<=attempts; i++)); do
    if systemctl is-active --quiet "$svc"; then return 0; fi
    sleep 1
  done
  return 1
}
for svc in rc-gateway.service scadaagent6.service scadaserver6.service scadacomm6.service scadaweb6.service nginx.service; do
  wait_service "$svc" 60 || die "serviço não ficou ativo: $svc" 7
done

curl -fsS --max-time 3 http://127.0.0.1:18080/readyz >/dev/null || die "Gateway não está ready após instalação" 7
rapid_web_ok=0
for ((i=1; i<=60; i++)); do
  if curl -fsSL --max-time 5 http://127.0.0.1:10008/ >/dev/null 2>&1 && curl -fsSL --max-time 5 http://127.0.0.1/ >/dev/null 2>&1; then
    rapid_web_ok=1
    break
  fi
  sleep 1
done
[[ $rapid_web_ok -eq 1 ]] || die "Rapid SCADA Webstation/Nginx não respondeu em loopback" 7

install -d -o root -g root -m 0750 "$STATE_DIR"
rapid_source_sha="$(sha256sum "$RAPID_SOURCE" | awk '{print $1}')"
cat > "$STATE_DIR/install-state.env" <<EOFSTATE
installed_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
gateway_version=$GATEWAY_VERSION
gateway_arch=$arch
gateway_sha256=$actual_gateway_sha
rapid_version=$RAPID_VERSION
rapid_source=$(basename "$RAPID_SOURCE")
rapid_source_sha256=$rapid_source_sha
rapid_deb_sha256=$RAPID_DEB_SHA256
webstation_bind=127.0.0.1:10008
nginx_bind=127.0.0.1:80
EOFSTATE
chmod 0640 "$STATE_DIR/install-state.env"

log "INSTALL OK: Gateway $GATEWAY_VERSION + Rapid SCADA $RAPID_VERSION"
log "Gateway admin: http://127.0.0.1:18080"
log "Rapid SCADA Web: http://127.0.0.1/ (loopback apenas)"
warn "O Rapid SCADA upstream usa credencial inicial documentada pelo fornecedor. Altere as credenciais antes de qualquer exposição de rede."
warn "Nenhum tunnel de campo é criado automaticamente quando rc-gateway.json não é fornecido. Configure a linha/controladora e então execute os acceptance tests Rapid SCADA."
