# Runbook operacional — RC Universal Gateway

> Estado técnico canônico: [`PROJECT_STATE.md`](./PROJECT_STATE.md). O Gateway é bridge-first: transporta dados; não interpreta registradores, não mantém historian e não habilita command plane genérico.

## Plataforma de referência

- Linux com systemd; Ubuntu Server 24.04 LTS é a plataforma inicial de homologação;
- usuário/grupo de serviço: `rc-gateway`;
- raiz: `/opt/rc-gateway`;
- configuração: `/etc/rc-gateway.json`;
- unit: `/etc/systemd/system/rc-gateway.service`;
- runtime sockets: `/run/rc-gateway`;
- release ativa: `/opt/rc-gateway/current`;
- release anterior: `/opt/rc-gateway/previous`.

A identidade antiga `rc-gateway-umbrella` não é mais o layout canônico pre-v1. O instalador recusa uma instalação legada detectada em vez de migrá-la silenciosamente.

## Validar configuração

```bash
/opt/rc-gateway/current/bin/rc-gateway \
  --check-config \
  --config /etc/rc-gateway.json
```

A configuração é fail-closed: JSON desconhecido/trailing é rejeitado; IDs/binds/sockets conflitantes são rejeitados; admin deve ser loopback; listeners TCP/UDP não-loopback exigem `allowedCidrs`; opções TLS exigem `tls.enabled=true`; mTLS listener exige CA; provider sockets devem usar o network correto; `unixpacket`↔stream exige `packetFraming: "length32be"`.

Recursos stream também são limitados:

```json
{
  "limits": {
    "maxActivePairs": 1024
  }
}
```

Cada tunnel possui `maxConcurrentPairs` (padrão `1`). Valor maior que `1` só é aceito em topologia com exatamente uma ponta `listen` e uma `connect`; `listen↔listen` paralelo é rejeitado para evitar pareamento temporal ambíguo entre clientes.

## Construir e validar release

Na raiz do repositório:

```bash
go install golang.org/x/vuln/cmd/govulncheck@8fcedea455d953a0f8470e1f41420bb6f2e72665
go install github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@07257d5b9cbd2a3d4338a880c0ca50081e1ac445

govulncheck ./...
bash scripts/test-installer-security.sh
REQUIRE_SBOM=1 ARCHES="amd64 arm64" \
  bash scripts/build-release.sh <versão>
```

A build gera `rc-gateway_<versão>_linux_<arch>.tar.gz`, SHA-256, binário versionado, `MANIFEST` com `product=rc-gateway`, SBOM CycloneDX, unit systemd, exemplos, scripts e documentação operacional. O pacote usa `-trimpath`, timestamp derivado do commit e tar/gzip determinísticos.

## Pré-validar pacote

```bash
bash scripts/install-release.sh --dry-run \
  dist/rc-gateway_<versão>_linux_amd64.tar.gz \
  dist/rc-gateway_<versão>_linux_amd64.tar.gz.sha256 \
  /caminho/config-candidata.json
```

O dry-run verifica SHA-256, estrutura do produto, rejeita path traversal, links e entradas especiais, exige uma única raiz de pacote e executa `--check-config` sem modificar o host.

## Instalar ou atualizar

```bash
sudo bash scripts/install-release.sh \
  dist/rc-gateway_<versão>_linux_amd64.tar.gz \
  dist/rc-gateway_<versão>_linux_amd64.tar.gz.sha256 \
  /caminho/config-candidata.json
```

O instalador cria usuário/grupo quando necessário, mantém releases imutáveis, limita backups de configuração, troca `current` atomicamente, reinicia e exige `systemctl is-active` + `/readyz`. Falha de readiness aciona rollback automático.

Parâmetros operacionais limitados:

```bash
RC_GATEWAY_HEALTH_ATTEMPTS=30          # 1..300
RC_GATEWAY_HEALTH_DELAY_SECONDS=1      # 1..60
RC_GATEWAY_CONFIG_BACKUPS=10           # 1..100
```

Os paths do produto são deliberadamente canônicos; não há override de root/config/unit que possa deixar a unit systemd apontando para um caminho diferente do instalador.

## systemd readiness e watchdog

A unit usa `Type=notify`, `NotifyAccess=main` e `WatchdogSec=30s`. O processo envia `READY=1` somente depois da barrier de readiness local do Gateway. Depois disso envia watchdog periódico usando `NOTIFY_SOCKET`. Se o processo continuar vivo mas deixar de cumprir o watchdog, systemd pode reiniciá-lo.

Readiness continua significando **infraestrutura local pronta**, não resposta do protocolo industrial remoto.

## Rollback

```bash
sudo /opt/rc-gateway/current/scripts/rollback-release.sh
```

O rollback exige `current` e `previous`, valida o `MANIFEST` do produto e a configuração com o binário anterior, troca atomicamente a release, reinicia e exige readiness. Se a release anterior também falhar, a tentativa é revertida.

## Observabilidade

Endpoints locais GET-only:

- `/healthz` — admin HTTP vivo;
- `/readyz` — componentes locais inicializados;
- `/status` — estado resumido;
- `/sessions` — sessões ativas;
- `/metrics` — Prometheus text format.

```bash
systemctl status rc-gateway.service --no-pager
journalctl -u rc-gateway.service -n 200 --no-pager
curl -fsS http://127.0.0.1:18080/healthz
curl -fsS http://127.0.0.1:18080/readyz
curl -fsS http://127.0.0.1:18080/status
curl -fsS http://127.0.0.1:18080/sessions
curl -fsS http://127.0.0.1:18080/metrics
```

Métricas de limites incluem `rc_gateway_max_active_pairs` e, por tunnel, `rc_gateway_tunnel_<id>_max_concurrent_pairs` e `_active`.

## Diagnóstico de campo

Crie um bundle local:

```bash
sudo /opt/rc-gateway/current/scripts/collect-diagnostics.sh
```

O bundle não copia a configuração, chaves nem `/sessions`, mas ainda pode conter IPs, IDs de dispositivos e linhas de journal. Revise/sanitize antes de compartilhar.

## VM acceptance

Depois de instalar o artifact exato produzido pelo CI:

```bash
sudo /opt/rc-gateway/current/scripts/vm-acceptance.sh
```

O script valida identidade de serviço, `Type=notify`, watchdog, configuração, health/readiness/status/métricas, restart controlado e exposição do admin padrão. O procedimento completo — incluindo upgrade/rollback, faults e soak — está em [`VM_ACCEPTANCE.md`](./VM_ACCEPTANCE.md).

## USB HID — descoberta e permissão segura

Com a controladora conectada:

```bash
bash /opt/rc-gateway/current/scripts/probe-usb-hid.sh
```

Registre `device`, VID/PID, serial, nome, interface, report descriptor/tamanho/SHA-256 e permissões. Prefira `vendorId` + `productId` + `serialNumber` quando houver serial estável; `/dev/hidrawN` pode mudar após boot/hotplug.

Exemplo de formato — substitua pelos valores reais:

```json
{
  "id": "comap-usb",
  "socket": "/run/rc-gateway/comap.hid.sock",
  "vendorId": "1234",
  "productId": "abcd",
  "serialNumber": "SERIAL-REAL",
  "maxReportBytes": 4096,
  "allowWrite": false
}
```

Não rode o daemon como root apenas para acessar hidraw. Use uma regra udev específica, por exemplo:

```text
SUBSYSTEM=="hidraw", ATTRS{idVendor}=="1234", ATTRS{idProduct}=="abcd", GROUP="rc-gateway", MODE="0660"
```

Evite regra genérica para todos os `hidraw`.

## `unixpacket` e framing para TCP

USB HID e CAN usam `unixpacket` porque fronteiras importam. Quando uma ponta packet cruza para stream, `packetFraming: "length32be"` codifica:

```text
4 bytes uint32 big-endian length | payload original
```

Isso é framing de transporte do Gateway, **não Modbus nem conversão de protocolo**.

## Segurança de campo

- `commandPlaneEnabled=true` é rejeitado;
- CAN TX permanece `allowTransmit=false` por padrão;
- USB HID write permanece `allowWrite=false` por padrão;
- listeners externos exigem CIDR allowlist;
- use VPN/firewall para equipamentos legados sem autenticação criptográfica;
- use TLS 1.3/mTLS quando possível;
- não conclua que escrita industrial está autorizada apenas porque o transporte funciona;
- consulte [`THREAT_MODEL.md`](./THREAT_MODEL.md) antes de introduzir nova superfície de ataque.

## Soak e impairment

O CI executa mini-soak. Na VM/HIL:

```bash
bash /opt/rc-gateway/current/scripts/run-soak.sh 86400   # 24 h
bash /opt/rc-gateway/current/scripts/run-soak.sh 604800  # 7 dias
```

Use `tc netem`/namespaces/firewall controlado para perda, jitter, atraso, resets e flapping. Registre RSS, CPU, FDs, sessões, erros, reconnects e readiness ao longo do tempo.

## HIL antes de `production validated`

Validar fisicamente somente os meios que serão declarados suportados: PUSR/USR/Teltonika, VPN/4G/MikroTik, RS232/422/485, USB HID/InteliLite 4 AMF 9, UDP quando aplicável, CAN/CAN-FD, power-cycle/reconnect, restart do consumidor/Gateway e rollback na máquina de homologação.

A InteliLite USB continua exigindo comprovação do protocolo de aplicação. Transporte HID funcionando não implica Modbus pela USB.

## Diagnóstico rápido

**Gateway não sobe:** execute `--check-config`, depois `systemctl status`/`journalctl`.

**`healthz` responde e `readyz` não:** algum componente local configurado não inicializou; verifique binds, sockets, HID e interface CAN.

**Muitas conexões:** confira `limits.maxActivePairs`, `maxConcurrentPairs`, `rc_gateway_active_pairs` e limites do host. Não aumente limites sem teste de carga.

**PUSR conectado sem comunicação:** confirme allowlist, porta, firewall/VPN, consumer e framing/protocolo esperado.

**Serial:** confirme device, permissões/dialout, baud/data/parity/stop bits e direção RS485.

**USB HID:** execute probe, valide seletor/udev, character device e protocolo/adapter de aplicação.

**CAN:** confirme interface UP, bitrate/FD e transceiver. Gateway preserva frames; não interpreta J1939/CANopen.

**Release nova falha:** o instalador deve executar rollback; confira `current`, `previous`, `MANIFEST`, unit e journal antes de nova tentativa.
