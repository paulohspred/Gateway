#!/usr/bin/env bash
set -Eeuo pipefail

ROOT="${RC_GATEWAY_ROOT:-/opt/rc-gateway}"
FRONTEND_ROOT="$ROOT/current/frontend"
NGINX_SITE="${RC_FRONTEND_NGINX_SITE:-/etc/nginx/sites-available/rc-scada}"
NGINX_LINK="${RC_FRONTEND_NGINX_LINK:-/etc/nginx/sites-enabled/rc-scada}"
STATE_DIR="${RC_SCADA_STATE_DIR:-/var/lib/rc-scada-stack}"
BIND="${RC_FRONTEND_BIND:-127.0.0.1:80}"
MONITOR="${RC_MONITOR_UPSTREAM:-127.0.0.1:18100}"
ADMIN="${RC_ADMIN_UPSTREAM:-127.0.0.1:18110}"
DRY_RUN=0

if [[ "${1:-}" == "--dry-run" ]]; then
  DRY_RUN=1
  shift
fi
[[ $# -eq 0 ]] || { echo "Uso: $0 [--dry-run]" >&2; exit 64; }

case "$BIND" in
  127.0.0.1:80|0.0.0.0:80) ;;
  *) echo "ERRO: RC_FRONTEND_BIND suportado: 127.0.0.1:80 ou 0.0.0.0:80" >&2; exit 64 ;;
esac
[[ "$MONITOR" =~ ^127\.0\.0\.1:[0-9]{1,5}$ ]] || { echo "ERRO: RC_MONITOR_UPSTREAM deve permanecer em loopback IPv4" >&2; exit 64; }
[[ "$ADMIN" =~ ^127\.0\.0\.1:[0-9]{1,5}$ ]] || { echo "ERRO: RC_ADMIN_UPSTREAM deve permanecer em loopback IPv4" >&2; exit 64; }

[[ -f "$FRONTEND_ROOT/index.html" ]] || { echo "ERRO: frontend de release ausente: $FRONTEND_ROOT/index.html" >&2; exit 4; }
[[ -d "$FRONTEND_ROOT/assets" ]] || { echo "ERRO: diretório de assets ausente: $FRONTEND_ROOT/assets" >&2; exit 4; }
find "$FRONTEND_ROOT/assets" -maxdepth 1 -type f -name 'index-*.js' -print -quit | grep -q . || { echo "ERRO: bundle JavaScript hashed ausente" >&2; exit 4; }

if [[ $DRY_RUN -eq 1 ]]; then
  echo "DRY-RUN OK: frontend=$FRONTEND_ROOT bind=$BIND monitor=$MONITOR admin=$ADMIN"
  exit 0
fi

[[ $EUID -eq 0 ]] || { echo "ERRO: instalação exige root." >&2; exit 77; }
for cmd in nginx systemctl curl find; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "ERRO: comando obrigatório ausente: $cmd" >&2; exit 69; }
done
wait_upstream(){
  local name="$1" url="$2"
  for _ in $(seq 1 30); do
    if curl -fsS --max-time 3 "$url" >/dev/null 2>&1; then return 0; fi
    sleep 1
  done
  echo "ERRO: $name não responde em $url após janela de readiness" >&2
  return 1
}
wait_upstream "RC Monitor" "http://$MONITOR/healthz"
wait_upstream "RC Admin" "http://$ADMIN/healthz"

install -d -o root -g root -m 0750 "$STATE_DIR/nginx-backups"
install -d -o root -g root -m 0755 "$(dirname "$NGINX_SITE")" "$(dirname "$NGINX_LINK")"

backup=""
had_site=0
if [[ -f "$NGINX_SITE" ]]; then
  had_site=1
  backup="$STATE_DIR/nginx-backups/rc-scada-$(date -u +%Y%m%dT%H%M%SZ)-$$.conf"
  cp --preserve=mode,ownership,timestamps "$NGINX_SITE" "$backup"
  chmod 0640 "$backup"
fi

listen_block='    listen 127.0.0.1:80 default_server;'
if [[ "$BIND" == "0.0.0.0:80" ]]; then
  listen_block=$'    listen 80 default_server;\n    listen [::]:80 default_server;'
fi

candidate="$(mktemp /etc/nginx/sites-available/.rc-scada.XXXXXX)"
cleanup(){ rm -f "$candidate"; }
trap cleanup EXIT
cat > "$candidate" <<NGINX
server {
$listen_block
    server_name _;

    root $FRONTEND_ROOT;
    index index.html;
    client_max_body_size 1m;

    add_header X-Content-Type-Options "nosniff" always;
    add_header X-Frame-Options "DENY" always;
    add_header Referrer-Policy "no-referrer" always;
    add_header Permissions-Policy "camera=(), microphone=(), geolocation=()" always;
    add_header Content-Security-Policy "default-src 'self'; base-uri 'self'; object-src 'none'; frame-ancestors 'none'; img-src 'self' data:; font-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'" always;

    location = /healthz {
        proxy_pass http://$MONITOR/healthz;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    location = /readyz {
        proxy_pass http://$MONITOR/readyz;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    location ^~ /api/v1/auth/ {
        proxy_pass http://$ADMIN;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    location ^~ /api/v1/admin/ {
        proxy_pass http://$ADMIN;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    location ^~ /api/v1/engineering/ {
        proxy_pass http://$ADMIN;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    location /api/ {
        proxy_pass http://$ADMIN;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    location /assets/ {
        try_files \$uri =404;
        expires 1y;
        add_header Cache-Control "public, immutable";
        add_header X-Content-Type-Options "nosniff" always;
    }

    location / {
        try_files \$uri \$uri/ /index.html;
        add_header Cache-Control "no-store";
        add_header X-Content-Type-Options "nosniff" always;
        add_header X-Frame-Options "DENY" always;
        add_header Referrer-Policy "no-referrer" always;
        add_header Permissions-Policy "camera=(), microphone=(), geolocation=()" always;
        add_header Content-Security-Policy "default-src 'self'; base-uri 'self'; object-src 'none'; frame-ancestors 'none'; img-src 'self' data:; font-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'" always;
    }
}
NGINX
chmod 0644 "$candidate"

rollback(){
  echo "ERRO: configuração frontend/Nginx falhou; restaurando site anterior." >&2
  if [[ $had_site -eq 1 && -n "$backup" && -f "$backup" ]]; then
    cp "$backup" "$NGINX_SITE"
    chmod 0644 "$NGINX_SITE"
    ln -sfn "$NGINX_SITE" "$NGINX_LINK"
  else
    rm -f "$NGINX_SITE" "$NGINX_LINK"
  fi
  nginx -t >/dev/null 2>&1 && systemctl restart nginx.service >/dev/null 2>&1 || true
}
trap rollback ERR

install -o root -g root -m 0644 "$candidate" "$NGINX_SITE"
ln -sfn "$NGINX_SITE" "$NGINX_LINK"
rm -f /etc/nginx/sites-enabled/default
nginx -t
systemctl restart nginx.service

ready=0
for _ in $(seq 1 30); do
  if systemctl is-active --quiet nginx.service \
     && curl -fsS --max-time 3 http://127.0.0.1/ >/dev/null \
     && curl -fsS --max-time 3 http://127.0.0.1/readyz >/dev/null; then
    ready=1
    break
  fi
  sleep 1
done
[[ $ready -eq 1 ]] || false
trap - ERR
trap cleanup EXIT

echo "INSTALL RC FRONTEND OK: root=$FRONTEND_ROOT bind=$BIND monitor=http://$MONITOR admin=http://$ADMIN"
