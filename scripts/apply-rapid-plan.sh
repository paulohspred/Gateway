#!/usr/bin/env bash
set -Eeuo pipefail

DRY_RUN=0
APPROVE=""
RAPID_ROOT="${RAPID_SCADA_ROOT:-/opt/scada}"
CONFIG_ROOT="$RAPID_ROOT/Config"
STATE_DIR="${RC_SCADA_STATE_DIR:-/var/lib/rc-scada-stack}"
SERVICES="${RAPID_PLAN_SERVICES:-scadaserver6.service scadacomm6.service}"
VALIDATOR="${RAPID_PLAN_VALIDATOR:-/opt/rc-gateway/current/scripts/rapid-scada-acceptance.sh}"

usage(){ cat >&2 <<'USAGE'
Uso: apply-rapid-plan.sh [--dry-run] [--approve APPLY] PLAN_DIR

Formato do plano:
  PLAN_DIR/VERSION
  PLAN_DIR/PLAN_SHA256
  PLAN_DIR/SHA256SUMS
  PLAN_DIR/files/<caminhos relativos sob /opt/scada/Config>

O hash do plano é SHA-256 de "version=<VERSION>\n" + SHA256SUMS.
Aplicação real exige root e --approve APPLY. O browser/RC Monitor não invocam esta ferramenta.
USAGE
exit "${1:-64}"; }

while (($#)); do
  case "$1" in
    --dry-run) DRY_RUN=1; shift;;
    --approve) [[ $# -ge 2 ]] || usage; APPROVE="$2"; shift 2;;
    -h|--help) usage 0;;
    --*) echo "ERRO: opção desconhecida: $1" >&2; usage;;
    *) [[ -z "${PLAN_DIR:-}" ]] || usage; PLAN_DIR="$1"; shift;;
  esac
done
[[ -n "${PLAN_DIR:-}" ]] || usage
PLAN_DIR="$(realpath "$PLAN_DIR")"

fail(){ echo "ERRO: $*" >&2; exit 1; }
for cmd in realpath sha256sum find sort comm awk grep cp install mkdir mv rm date; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "ERRO: comando obrigatório ausente: $cmd" >&2; exit 69; }
done
[[ -d "$PLAN_DIR/files" && -f "$PLAN_DIR/VERSION" && -f "$PLAN_DIR/SHA256SUMS" && -f "$PLAN_DIR/PLAN_SHA256" ]] || fail "plano incompleto"
[[ -d "$CONFIG_ROOT" ]] || fail "Rapid Config root ausente: $CONFIG_ROOT"

version="$(tr -d '\r\n' < "$PLAN_DIR/VERSION")"
[[ "$version" =~ ^[A-Za-z0-9._+-]{1,80}$ ]] || fail "VERSION inválida"
expected_plan_hash="$(tr -d '[:space:]' < "$PLAN_DIR/PLAN_SHA256")"
[[ "$expected_plan_hash" =~ ^[0-9a-fA-F]{64}$ ]] || fail "PLAN_SHA256 inválido"

if find "$PLAN_DIR/files" -type l -print -quit | grep -q .; then
  fail "symlinks não são permitidos em files/"
fi

manifest_paths="$(mktemp)"
actual_paths="$(mktemp)"
cleanup_tmp(){ rm -f "$manifest_paths" "$actual_paths"; }
trap cleanup_tmp EXIT

while IFS= read -r line; do
  [[ -n "$line" ]] || continue
  hash="${line%% *}"
  rel="${line#*  }"
  [[ "$hash" =~ ^[0-9a-fA-F]{64}$ && "$rel" != "$line" ]] || fail "linha inválida em SHA256SUMS"
  [[ "$rel" =~ ^[A-Za-z0-9._/-]+$ ]] || fail "caminho não permitido no plano: $rel"
  [[ "$rel" != /* ]] || fail "caminho absoluto não permitido: $rel"
  IFS='/' read -r -a parts <<<"$rel"
  for part in "${parts[@]}"; do
    [[ -n "$part" && "$part" != "." && "$part" != ".." ]] || fail "path traversal não permitido: $rel"
  done
  [[ -f "$PLAN_DIR/files/$rel" && ! -L "$PLAN_DIR/files/$rel" ]] || fail "arquivo do manifest ausente/não regular: $rel"
  printf '%s\n' "$rel" >> "$manifest_paths"
done < "$PLAN_DIR/SHA256SUMS"
[[ -s "$manifest_paths" ]] || fail "SHA256SUMS vazio"

( cd "$PLAN_DIR/files" && sha256sum -c "$PLAN_DIR/SHA256SUMS" >/dev/null ) || fail "checksum de arquivo do plano inválido"
find "$PLAN_DIR/files" -type f -printf '%P\n' | LC_ALL=C sort > "$actual_paths"
LC_ALL=C sort -u "$manifest_paths" -o "$manifest_paths"
[[ ! -s <(comm -3 "$manifest_paths" "$actual_paths") ]] || { comm -3 "$manifest_paths" "$actual_paths" >&2; fail "files/ e SHA256SUMS divergem"; }

computed_plan_hash="$({ printf 'version=%s\n' "$version"; cat "$PLAN_DIR/SHA256SUMS"; } | sha256sum | awk '{print $1}')"
[[ "${computed_plan_hash,,}" == "${expected_plan_hash,,}" ]] || fail "PLAN_SHA256 não corresponde ao conteúdo"

printf 'Rapid plan validado: version=%s sha256=%s files=%s\n' "$version" "$computed_plan_hash" "$(wc -l < "$manifest_paths")"
if [[ $DRY_RUN -eq 1 ]]; then
  echo "DRY-RUN OK: nenhum arquivo ou serviço foi alterado"
  exit 0
fi

[[ $EUID -eq 0 ]] || { echo "ERRO: aplicação real exige root" >&2; exit 77; }
[[ "$APPROVE" == "APPLY" ]] || fail "aplicação real exige --approve APPLY"
command -v systemctl >/dev/null 2>&1 || { echo "ERRO: systemctl ausente" >&2; exit 69; }

install -d -o root -g root -m 0750 "$STATE_DIR/rapid-plan-staging" "$STATE_DIR/rapid-plan-snapshots"
stage="$STATE_DIR/rapid-plan-staging/$computed_plan_hash"
rm -rf "$stage"
install -d -o root -g root -m 0750 "$stage/files"
cp -a "$PLAN_DIR/files/." "$stage/files/"
cp "$PLAN_DIR/VERSION" "$PLAN_DIR/PLAN_SHA256" "$PLAN_DIR/SHA256SUMS" "$stage/"
chmod -R go-w "$stage"

stamp="$(date -u +%Y%m%dT%H%M%SZ)"
snapshot="$STATE_DIR/rapid-plan-snapshots/${stamp}-${computed_plan_hash:0:12}"
install -d -o root -g root -m 0750 "$snapshot/previous"
: > "$snapshot/state.tsv"

while IFS= read -r rel; do
  target="$CONFIG_ROOT/$rel"
  if [[ -L "$target" ]]; then fail "target symlink recusado: $target"; fi
  if [[ -e "$target" ]]; then
    [[ -f "$target" ]] || fail "target existente não é arquivo regular: $target"
    install -d -o root -g root -m 0750 "$snapshot/previous/$(dirname "$rel")"
    cp -a "$target" "$snapshot/previous/$rel"
    printf 'present\t%s\n' "$rel" >> "$snapshot/state.tsv"
  else
    printf 'absent\t%s\n' "$rel" >> "$snapshot/state.tsv"
  fi
done < "$manifest_paths"

rollback(){
  rc=$?
  echo "ERRO: Rapid plan falhou; restaurando snapshot $snapshot" >&2
  while IFS=$'\t' read -r state rel; do
    target="$CONFIG_ROOT/$rel"
    case "$state" in
      present)
        install -d -o root -g root -m 0755 "$(dirname "$target")"
        cp -a "$snapshot/previous/$rel" "$target"
        ;;
      absent) rm -f "$target";;
    esac
  done < "$snapshot/state.tsv"
  systemctl restart $SERVICES >/dev/null 2>&1 || true
  exit "$rc"
}
trap rollback ERR

while IFS= read -r rel; do
  target="$CONFIG_ROOT/$rel"
  install -d -o root -g root -m 0755 "$(dirname "$target")"
  tmp="$(mktemp "$(dirname "$target")/.rapid-plan.XXXXXX")"
  install -o root -g root -m 0644 "$stage/files/$rel" "$tmp"
  mv -f "$tmp" "$target"
done < "$manifest_paths"

systemctl restart $SERVICES
for svc in $SERVICES; do
  ready=0
  for _ in $(seq 1 45); do
    if systemctl is-active --quiet "$svc"; then ready=1; break; fi
    sleep 1
  done
  [[ $ready -eq 1 ]] || fail "$svc não recuperou após Rapid plan"
done

web_config="$RAPID_ROOT/ScadaWeb/config/ScadaWebConfig.xml"
if [[ -f "$web_config" ]]; then
  grep -Eq '<Option name="AllowCommandApi" value="false"[[:space:]]*/>' "$web_config" || fail "Rapid Command API não está explicitamente false"
fi
if [[ -n "$VALIDATOR" && -x "$VALIDATOR" ]]; then
  "$VALIDATOR"
fi

trap - ERR
actor="${SUDO_USER:-root}"
cat > "$snapshot/APPLY_EVIDENCE" <<EVIDENCE
version=$version
planSha256=$computed_plan_hash
appliedAt=$stamp
actor=$actor
configRoot=$CONFIG_ROOT
services=$SERVICES
validator=$VALIDATOR
result=PASS
EVIDENCE
chmod 0640 "$snapshot/APPLY_EVIDENCE"
printf 'RAPID PLAN APPLY PASSED: version=%s sha256=%s snapshot=%s actor=%s\n' "$version" "$computed_plan_hash" "$snapshot" "$actor"
