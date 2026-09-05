# Runbook operacional — RC Universal Gateway

> Documento operacional da release bridge-first. O estado técnico canônico continua em [`PROJECT_STATE.md`](./PROJECT_STATE.md).

## 1. Princípio operacional

O RC Universal Gateway é uma **ponte de conectividade**. Ele não possui banco de registradores, historian de telemetria nem lógica de controladora.

```text
FIELD / MODEM / SERIAL / CAN
          ↓
   RC UNIVERSAL GATEWAY
          ↓
 RAPID / FUXA / SOFTWARE DO FABRICANTE / OUTRO
```

Regra fixa:

```text
BRIDGE FIRST
PROTOCOL OPTIONAL
NO DEVICE MEMORY DATABASE
NO TELEMETRY HISTORIAN
```

## 2. Plataforma de referência

- Linux com systemd; Ubuntu 24.04 é a plataforma inicial de homologação;
- usuário de serviço: `rc-gateway`;
- raiz standalone: `/opt/rc-gateway-umbrella`;
- configuração: `/etc/rc-gateway-umbrella.json`;
- unit: `/etc/systemd/system/rc-gateway-umbrella.service`;
- runtime sockets: `/run/rc-gateway`;
- releases imutáveis: `/opt/rc-gateway-umbrella/releases/<versão>`;
- release ativa: `/opt/rc-gateway-umbrella/current`;
- release anterior: `/opt/rc-gateway-umbrella/previous`.

## 3. Validar configuração antes de qualquer mudança

```bash
/opt/rc-gateway-umbrella/current/bin/rc-gateway \
  --check-config \
  --config /etc/rc-gateway-umbrella.json
```

O parser é estrito: campos JSON desconhecidos, documentos extras, IDs/binds conflitantes, sockets de provider conflitantes e outras colisões conhecidas são rejeitados antes de abrir transports.

## 4. Construir uma release

Na raiz `gateway-umbrella/`:

```bash
go install golang.org/x/vuln/cmd/govulncheck@8fcedea455d953a0f8470e1f41420bb6f2e72665
go install github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@v1.12.0

govulncheck ./...
REQUIRE_SBOM=1 ARCHES="amd64 arm64" \
  bash scripts/build-release.sh <versão>
```

O `govulncheck` está pinado nessa revisão upstream por compatibilidade com Go 1.27.1. Não substituir por uma versão antiga sem repetir o gate de vulnerabilidade.

A build gera, por arquitetura:

- `rc-gateway` com versão/commit/build embutidos;
- `MANIFEST`;
- `VERSION`;
- SBOM CycloneDX JSON;
- unit systemd;
- exemplos de configuração;
- instalador e rollback;
- `.tar.gz`;
- `.sha256`.

A build usa `-trimpath`, `-buildvcs=false`, timestamp derivado do commit e tar/gzip determinísticos para permitir verificação de reprodutibilidade.

## 5. Pré-validar um pacote sem modificar a máquina

```bash
bash scripts/install-release.sh --dry-run \
  dist/rc-gateway-umbrella_<versão>_linux_amd64.tar.gz \
  dist/rc-gateway-umbrella_<versão>_linux_amd64.tar.gz.sha256 \
  /caminho/config-candidata.json
```

O dry-run verifica SHA256, paths do tar, estrutura do pacote, executabilidade, versão e `--check-config`. Nenhum serviço ou diretório de produção é alterado.

## 6. Instalar ou atualizar

```bash
sudo bash scripts/install-release.sh \
  dist/rc-gateway-umbrella_<versão>_linux_amd64.tar.gz \
  dist/rc-gateway-umbrella_<versão>_linux_amd64.tar.gz.sha256 \
  /caminho/config-candidata.json
```

O instalador:

1. verifica SHA256;
2. rejeita path traversal no arquivo;
3. extrai em staging;
4. valida a configuração com o binário novo;
5. cria usuário/grupo de serviço quando necessário;
6. preserva a configuração anterior;
7. instala a release como diretório imutável;
8. atualiza `previous`;
9. troca `current` atomicamente;
10. instala/recarrega a unit;
11. reinicia o serviço;
12. exige `systemctl is-active` + `/readyz`;
13. se a nova release não ficar saudável, restaura binário e configuração anteriores automaticamente.

Se o admin bind não for `127.0.0.1:18080`, definir antes da instalação:

```bash
export RC_GATEWAY_HEALTH_URL='http://IP:PORTA/readyz'
```

## 7. Rollback manual

```bash
sudo /opt/rc-gateway-umbrella/current/scripts/rollback-release.sh
```

O rollback valida a configuração com o binário anterior, troca `current`, reinicia, verifica readiness e reverte a própria tentativa se a release anterior também não ficar saudável.

## 8. Observabilidade

Endpoints administrativos:

- `/healthz` — processo HTTP vivo;
- `/readyz` — runtime pronto;
- `/status` — estado resumido;
- `/sessions` — sessões/túneis ativos;
- `/metrics` — métricas Prometheus/OpenMetrics text format.

Comandos operacionais:

```bash
systemctl status rc-gateway-umbrella --no-pager
journalctl -u rc-gateway-umbrella -n 200 --no-pager
curl -fsS http://127.0.0.1:18080/healthz
curl -fsS http://127.0.0.1:18080/readyz
curl -fsS http://127.0.0.1:18080/status
curl -fsS http://127.0.0.1:18080/sessions
curl -fsS http://127.0.0.1:18080/metrics
```

## 9. Segurança de campo

- `commandPlaneEnabled=true` é rejeitado;
- CAN TX é bloqueado por padrão (`allowTransmit=false`);
- listeners publicados devem usar allowlist CIDR quando a política exigir;
- preferir VPN/firewall de borda para equipamento legado que não suporte TLS;
- usar TLS 1.3/mTLS quando os dois extremos suportarem;
- chave/certificado TLS deve ser legível pelo usuário/grupo `rc-gateway`;
- não expor endpoint administrativo diretamente à Internet;
- manter firewall da MikroTik/host restritivo;
- não habilitar escrita industrial por assumir que “o transporte funciona”.

## 10. Soak e impairment

CI executa mini-soak com fragmentação, latência/jitter e reconnect. Na VM:

```bash
bash scripts/run-soak.sh 86400
```

para 24 h, e:

```bash
bash scripts/run-soak.sh 604800
```

para 7 dias.

Para homologação de rede real, adicionar `tc netem` em interface/namespace de laboratório para perda, jitter, atraso, duplicação e flapping. O mini-soak em user-space não substitui esse gate físico.

## 11. Checklist HIL obrigatório antes de “produção validada”

- PUSR/USR real em reverse TCP → Gateway → Rapid SCADA;
- dispositivo direto por IP/VPN → Gateway → consumidor;
- RS232 real;
- RS422 real quando houver hardware;
- RS485 real, incluindo direção/half-duplex do adaptador utilizado;
- UDP real quando houver dispositivo desse tipo;
- CAN clássico em interface física;
- CAN-FD em interface física compatível;
- queda/reconexão de 4G/VPN;
- power-cycle de modem/controladora;
- restart do consumidor;
- restart do Gateway;
- 24 h sem crescimento anormal de memória/FD/goroutines;
- alvo de 7 dias de soak;
- rollback testado em máquina de homologação.

Somente depois desses gates físicos o estado pode mudar de **software field-test-ready** para **production validated**.

## 12. Diagnóstico rápido

**Gateway não sobe:** executar `--check-config`, depois consultar `journalctl`.

**`healthz` responde e `readyz` não:** runtime iniciou mas algum provider/túnel obrigatório não chegou ao estado pronto; consultar logs/status.

**PUSR conectado mas Rapid não comunica:** conferir pareamento da sessão, destino local, firewall e se o consumidor abriu a outra ponta. Não alterar payload para “corrigir” protocolo.

**Serial sem comunicação:** conferir `/dev/tty*`, grupo `dialout`, baud/data/parity/stop bits e direção RS485 do hardware.

**CAN sem comunicação:** conferir interface `ip -details link show`, estado UP, bitrate/FD do kernel e transceiver. J1939/CANopen não são interpretados pelo Gateway.

**Release nova falha:** o instalador tenta rollback automaticamente; confirmar `current`, `previous`, status systemd e logs antes de nova tentativa.
