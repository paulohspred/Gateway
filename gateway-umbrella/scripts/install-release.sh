#!/usr/bin/env bash
set -Eeuo pipefail
SERVICE="${RC_GATEWAY_SERVICE:-rc-gateway-umbrella.service}"
ROOT="${RC_GATEWAY_ROOT:-/opt/rc-gateway-umbrella}"
CONFIG_TARGET="${RC_GATEWAY_CONFIG:-/etc/rc-gateway-umbrella.json}"
UNIT_TARGET="${RC_GATEWAY_UNIT:-/etc/systemd/system/rc-gateway-umbrella.service}"
HEALTH_URL="${RC_GATEWAY_HEALTH_URL:-http://127.0.0.1:18080/readyz}"
HEALTH_ATTEMPTS="${RC_GATEWAY_HEALTH_ATTEMPTS:-30}"
HEALTH_DELAY="${RC_GATEWAY_HEALTH_DELAY_SECONDS:-1}"
usage(){ echo "Uso: $0 [--dry-run] ARQUIVO.tar.gz ARQUIVO.sha256 [CONFIG_CANDIDATA.json]" >&2; exit 64; }
DRY_RUN=0
if [[ "${1:-}" == "--dry-run" ]]; then DRY_RUN=1; shift; fi
[[ $# -ge 2 && $# -le 3 ]] || usage
ARCHIVE="$(realpath "$1")"
CHECKSUM="$(realpath "$2")"
CANDIDATE_CONFIG="${3:-$CONFIG_TARGET}"
[[ -f "$ARCHIVE" && -f "$CHECKSUM" ]] || { echo "ERRO: release/checksum ausente." >&2; exit 2; }
[[ -f "$CANDIDATE_CONFIG" ]] || { echo "ERRO: configuração candidata ausente: $CANDIDATE_CONFIG" >&2; exit 2; }
expected="$(awk 'NF {print $1; exit}' "$CHECKSUM")"
actual="$(sha256sum "$ARCHIVE" | awk '{print $1}')"
[[ "$expected" =~ ^[0-9a-fA-F]{64}$ ]] || { echo "ERRO: checksum inválido." >&2; exit 2; }
[[ "${actual,,}" == "${expected,,}" ]] || { echo "ERRO: SHA256 da release não confere." >&2; exit 3; }
tmp="$(mktemp -d)"
cleanup(){ rm -rf "$tmp"; }
trap cleanup EXIT
if tar -tzf "$ARCHIVE" | grep -Eq '(^/|(^|/)\.\.(/|$))'; then echo "ERRO: release contém caminho inseguro." >&2; exit 4; fi
tar -xzf "$ARCHIVE" -C "$tmp"
mapfile -t roots < <(find "$tmp" -mindepth 1 -maxdepth 1 -type d -print)
[[ ${#roots[@]} -eq 1 ]] || { echo "ERRO: release deve conter exatamente um diretório raiz." >&2; exit 4; }
pkg="${roots[0]}"
[[ -x "$pkg/bin/rc-gateway" ]] || { echo "ERRO: binário rc-gateway ausente/inexecutável." >&2; exit 4; }
[[ -f "$pkg/systemd/rc-gateway-umbrella.service" ]] || { echo "ERRO: unit systemd ausente." >&2; exit 4; }
[[ -f "$pkg/VERSION" && -f "$pkg/MANIFEST" ]] || { echo "ERRO: metadados de release ausentes." >&2; exit 4; }
version="$(tr -d '\r\n' < "$pkg/VERSION")"
[[ "$version" =~ ^[A-Za-z0-9._+-]+$ ]] || { echo "ERRO: versão insegura: $version" >&2; exit 4; }
"$pkg/bin/rc-gateway" --check-config --config "$CANDIDATE_CONFIG"
"$pkg/bin/rc-gateway" --version
if [[ $DRY_RUN -eq 1 ]]; then echo "DRY-RUN OK: release=$version sha256=$actual config=$CANDIDATE_CONFIG"; exit 0; fi
[[ $EUID -eq 0 ]] || { echo "ERRO: instalação exige root." >&2; exit 77; }
command -v systemctl >/dev/null || { echo "ERRO: systemctl não encontrado." >&2; exit 69; }
command -v curl >/dev/null || { echo "ERRO: curl não encontrado." >&2; exit 69; }
getent group rc-gateway >/dev/null || groupadd --system rc-gateway
if ! id rc-gateway >/dev/null 2>&1; then useradd --system --gid rc-gateway --home-dir /nonexistent --shell /usr/sbin/nologin rc-gateway; fi
if getent group dialout >/dev/null; then usermod -a -G dialout rc-gateway; fi
install -d -o root -g root -m 0755 "$ROOT" "$ROOT/releases"
config_dir="$(dirname "$CONFIG_TARGET")"
if [[ ! -d "$config_dir" ]]; then install -d -o root -g root -m 0755 "$config_dir"; fi
release_dir="$ROOT/releases/$version"
if [[ -e "$release_dir" ]]; then
  [[ -f "$release_dir/MANIFEST" ]] || { echo "ERRO: release existente sem MANIFEST: $release_dir" >&2; exit 5; }
  old_commit="$(awk -F= '$1=="commit"{print $2}' "$release_dir/MANIFEST")"
  new_commit="$(awk -F= '$1=="commit"{print $2}' "$pkg/MANIFEST")"
  [[ -n "$old_commit" && "$old_commit" == "$new_commit" ]] || { echo "ERRO: versão $version já existe com conteúdo diferente." >&2; exit 5; }
else
  mv "$pkg" "$release_dir"
fi
chown -R root:root "$release_dir"
find "$release_dir" -type d -exec chmod 0755 {} +
chmod 0755 "$release_dir/bin/rc-gateway" "$release_dir/scripts/"*.sh 2>/dev/null || true
find "$release_dir" -type f ! -path '*/bin/rc-gateway' ! -path '*/scripts/*.sh' -exec chmod 0644 {} +
old_current=""
if [[ -L "$ROOT/current" ]]; then old_current="$(readlink -f "$ROOT/current" || true)"; fi
config_backup=""
had_config=0
if [[ -f "$CONFIG_TARGET" ]]; then
  had_config=1
  install -d -o root -g root -m 0700 "$ROOT/config-backups"
  config_backup="$ROOT/config-backups/config-$(date -u +%Y%m%dT%H%M%SZ)-$$.json"
  cp --preserve=mode,ownership,timestamps "$CONFIG_TARGET" "$config_backup"
fi
config_tmp="$(mktemp "$(dirname "$CONFIG_TARGET")/.rc-gateway-config.XXXXXX")"
install -o root -g rc-gateway -m 0640 "$CANDIDATE_CONFIG" "$config_tmp"
mv -f "$config_tmp" "$CONFIG_TARGET"
install -o root -g root -m 0644 "$release_dir/systemd/rc-gateway-umbrella.service" "$UNIT_TARGET"
if [[ -n "$old_current" ]]; then ln -sfn "$old_current" "$ROOT/previous.new"; mv -Tf "$ROOT/previous.new" "$ROOT/previous"; fi
ln -sfn "$release_dir" "$ROOT/current.new"
mv -Tf "$ROOT/current.new" "$ROOT/current"
rollback(){
  echo "ERRO: nova release não ficou saudável; executando rollback." >&2
  if [[ -n "$old_current" && -d "$old_current" ]]; then ln -sfn "$old_current" "$ROOT/current.rollback"; mv -Tf "$ROOT/current.rollback" "$ROOT/current"; else rm -f "$ROOT/current"; fi
  if [[ $had_config -eq 1 && -n "$config_backup" && -f "$config_backup" ]]; then cp --preserve=mode,ownership,timestamps "$config_backup" "$CONFIG_TARGET"; elif [[ $had_config -eq 0 ]]; then rm -f "$CONFIG_TARGET"; fi
  systemctl daemon-reload || true
  if [[ -n "$old_current" ]]; then systemctl restart "$SERVICE" || true; else systemctl stop "$SERVICE" || true; fi
}
trap rollback ERR
systemctl daemon-reload
systemctl enable "$SERVICE" >/dev/null
systemctl restart "$SERVICE"
healthy=0
for ((i=1; i<=HEALTH_ATTEMPTS; i++)); do
  if systemctl is-active --quiet "$SERVICE" && curl -fsS --max-time 2 "$HEALTH_URL" >/dev/null; then healthy=1; break; fi
  sleep "$HEALTH_DELAY"
done
[[ $healthy -eq 1 ]] || false
trap - ERR
echo "INSTALL OK: release=$version current=$release_dir health=$HEALTH_URL"
