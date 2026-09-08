#!/usr/bin/env bash
set -Eeuo pipefail

CONFIG="${RAPID_SCADA_WEB_CONFIG:-/opt/scada/ScadaWeb/config/ScadaWebConfig.xml}"
SERVICE="${RAPID_SCADA_WEB_SERVICE:-scadaweb6.service}"
STATE_DIR="${RC_SCADA_STATE_DIR:-/var/lib/rc-scada-stack}"
DRY_RUN=0

if [[ "${1:-}" == "--dry-run" ]]; then
  DRY_RUN=1
  shift
fi
[[ $# -eq 0 ]] || { echo "Uso: $0 [--dry-run]" >&2; exit 64; }

for cmd in python3 grep; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "ERRO: comando obrigatório ausente: $cmd" >&2; exit 69; }
done
[[ -f "$CONFIG" ]] || { echo "ERRO: ScadaWebConfig.xml ausente: $CONFIG" >&2; exit 2; }

auth_count="$(grep -c 'name="AllowAuthApi"' "$CONFIG" || true)"
command_count="$(grep -c 'name="AllowCommandApi"' "$CONFIG" || true)"
[[ "$auth_count" == "1" ]] || { echo "ERRO: esperado exatamente um AllowAuthApi; encontrado $auth_count" >&2; exit 4; }
[[ "$command_count" == "1" ]] || { echo "ERRO: esperado exatamente um AllowCommandApi; encontrado $command_count" >&2; exit 4; }

if [[ $DRY_RUN -eq 1 ]]; then
  echo "DRY-RUN OK: será habilitado AllowAuthApi=true e mantido AllowCommandApi=false em $CONFIG"
  exit 0
fi

[[ $EUID -eq 0 ]] || { echo "ERRO: configuração exige root." >&2; exit 77; }
command -v systemctl >/dev/null 2>&1 || { echo "ERRO: systemctl ausente" >&2; exit 69; }
command -v curl >/dev/null 2>&1 || { echo "ERRO: curl ausente" >&2; exit 69; }

install -d -o root -g root -m 0750 "$STATE_DIR"
backup="$STATE_DIR/ScadaWebConfig.before-rc-monitor-api.xml"
if [[ ! -f "$backup" ]]; then
  cp --preserve=mode,ownership,timestamps "$CONFIG" "$backup"
  chmod 0640 "$backup"
fi

python3 - "$CONFIG" <<'PY'
import os
import re
import shutil
import stat
import sys
import tempfile

path = sys.argv[1]
with open(path, "rb") as f:
    data = f.read()

rules = [
    (rb'(<Option\s+name="AllowAuthApi"\s+value=")[^"]*("\s*/>)', rb'\1true\2', "AllowAuthApi"),
    (rb'(<Option\s+name="AllowCommandApi"\s+value=")[^"]*("\s*/>)', rb'\1false\2', "AllowCommandApi"),
]
for pattern, replacement, name in rules:
    data, count = re.subn(pattern, replacement, data)
    if count != 1:
        raise SystemExit(f"expected exactly one {name} option, replaced {count}")

st = os.stat(path)
dirname = os.path.dirname(path)
fd, tmp = tempfile.mkstemp(prefix=".ScadaWebConfig.", dir=dirname)
try:
    with os.fdopen(fd, "wb") as f:
        f.write(data)
        f.flush()
        os.fsync(f.fileno())
    os.chmod(tmp, stat.S_IMODE(st.st_mode))
    os.chown(tmp, st.st_uid, st.st_gid)
    os.replace(tmp, path)
finally:
    if os.path.exists(tmp):
        os.unlink(tmp)
PY

grep -Eq '<Option name="AllowAuthApi" value="true"[[:space:]]*/>' "$CONFIG" || { echo "ERRO: AllowAuthApi não ficou true" >&2; exit 5; }
grep -Eq '<Option name="AllowCommandApi" value="false"[[:space:]]*/>' "$CONFIG" || { echo "ERRO: AllowCommandApi não ficou false" >&2; exit 5; }

systemctl restart "$SERVICE"
ready=0
for _ in $(seq 1 30); do
  if systemctl is-active --quiet "$SERVICE" && curl -fsSL --max-time 3 http://127.0.0.1:10008/ >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 1
done
[[ $ready -eq 1 ]] || { echo "ERRO: $SERVICE não recuperou após configurar Web API" >&2; exit 6; }

echo "RAPID WEB API OK: auth=true command=false bind=127.0.0.1:10008"
