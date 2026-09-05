# Runbook operacional — RC Universal Gateway

> O estado técnico canônico está em [`PROJECT_STATE.md`](./PROJECT_STATE.md). O Gateway é bridge-first: transporta dados; não interpreta registradores, não mantém historian e não habilita command plane genérico.

## Plataforma de referência

- Linux com systemd; Ubuntu 24.04 é a plataforma inicial de homologação;
- usuário de serviço: `rc-gateway`;
- raiz de instalação: `/opt/rc-gateway-umbrella`;
- configuração: `/etc/rc-gateway-umbrella.json`;
- unit: `/etc/systemd/system/rc-gateway-umbrella.service`;
- runtime sockets: `/run/rc-gateway`;
- release ativa: `/opt/rc-gateway-umbrella/current`;
- release anterior: `/opt/rc-gateway-umbrella/previous`.

## Validar configuração

Antes de instalar, reiniciar ou alterar topologia:

```bash
/opt/rc-gateway-umbrella/current/bin/rc-gateway \
  --check-config \
  --config /etc/rc-gateway-umbrella.json
```

A configuração é fail-closed: JSON desconhecido/trailing é rejeitado; IDs/binds/sockets conflitantes são rejeitados; admin deve ser loopback; listeners TCP/UDP não-loopback exigem `allowedCidrs`; opções TLS exigem `tls.enabled=true`; mTLS listener exige CA; provider sockets devem usar o network correto; e `unixpacket`↔stream exige `packetFraming: "length32be"`.

`LoadStrict` valida a mesma fotografia de bytes lida do arquivo, evitando que uma segunda leitura use conteúdo diferente durante o check.

## Construir release

Na **raiz deste repositório**:

```bash
go install golang.org/x/vuln/cmd/govulncheck@8fcedea455d953a0f8470e1f41420bb6f2e72665
go install github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@v1.12.0

govulncheck ./...
bash scripts/test-installer-security.sh
REQUIRE_SBOM=1 ARCHES="amd64 arm64" \
  bash scripts/build-release.sh <versão>
```

A build gera binário versionado, `MANIFEST`, `VERSION`, SBOM CycloneDX, unit systemd, exemplos, scripts de instalação/rollback, `.tar.gz` e `.sha256`. O pacote usa `-trimpath`, timestamp derivado do commit e tar/gzip determinísticos.

## Pré-validar pacote

```bash
bash scripts/install-release.sh --dry-run \
  dist/rc-gateway-umbrella_<versão>_linux_amd64.tar.gz \
  dist/rc-gateway-umbrella_<versão>_linux_amd64.tar.gz.sha256 \
  /caminho/config-candidata.json
```

O dry-run verifica SHA256, rejeita path traversal, symlinks, hardlinks e entradas especiais, exige uma única raiz de pacote, valida estrutura/executabilidade/versão e executa `--check-config` sem modificar o host.

## Instalar ou atualizar

```bash
sudo bash scripts/install-release.sh \
  dist/rc-gateway-umbrella_<versão>_linux_amd64.tar.gz \
  dist/rc-gateway-umbrella_<versão>_linux_amd64.tar.gz.sha256 \
  /caminho/config-candidata.json
```

O instalador valida pacote e configuração, cria usuário/grupo quando necessário, mantém releases imutáveis, preserva configuração anterior, limita backups de configuração (10 por padrão; `RC_GATEWAY_CONFIG_BACKUPS` permite 1..100), troca `current` atomicamente, reinicia e exige `systemctl is-active` + `/readyz`. Falha de readiness aciona rollback automático.

Parâmetros operacionais opcionais possuem limites explícitos:

```bash
RC_GATEWAY_HEALTH_ATTEMPTS=30          # 1..300
RC_GATEWAY_HEALTH_DELAY_SECONDS=1      # 1..60
RC_GATEWAY_CONFIG_BACKUPS=10           # 1..100
```

O endpoint administrativo é **loopback-only** nesta release. Não altere a configuração para expô-lo. Para administração remota, use acesso ao host via VPN/SSH e, quando necessário, forwarding para `127.0.0.1:18080`.

## Rollback

```bash
sudo /opt/rc-gateway-umbrella/current/scripts/rollback-release.sh
```

O rollback valida a configuração com o binário anterior, troca `current`, reinicia, exige readiness e desfaz a própria tentativa se a release anterior também não ficar saudável.

## Observabilidade

Endpoints locais GET-only:

- `/healthz` — servidor administrativo vivo;
- `/readyz` — componentes locais configurados inicializados;
- `/status` — estado resumido;
- `/sessions` — sessões ativas;
- `/metrics` — métricas Prometheus text format.

```bash
systemctl status rc-gateway-umbrella --no-pager
journalctl -u rc-gateway-umbrella -n 200 --no-pager
curl -fsS http://127.0.0.1:18080/healthz
curl -fsS http://127.0.0.1:18080/readyz
curl -fsS http://127.0.0.1:18080/status
curl -fsS http://127.0.0.1:18080/sessions
curl -fsS http://127.0.0.1:18080/metrics
```

`readyz=ready` significa que admin/listeners/providers locais inicializaram. Para HID, também significa que o equipamento configurado foi resolvido e o character device foi validado. Não significa que PLC, modem ou protocolo de aplicação respondeu. Esse estado é comprovado somente por HIL.

## USB HID — descoberta e permissão segura

Com a controladora conectada ao Linux:

```bash
bash scripts/probe-usb-hid.sh
```

Registre pelo menos:

```text
device
vendor_id
product_id
hid_serial / usb_serial
hid_name / usb_product_name
interface
report_descriptor_bytes
report_descriptor_sha256
permissions
```

Em produção, prefira `vendorId` + `productId` + `serialNumber` quando o equipamento fornecer serial estável. `/dev/hidrawN` pode mudar após boot/hotplug.

Exemplo de configuração — substitua pelos valores reais do probe:

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

Os IDs acima são apenas exemplo de formato.

### udev / least privilege

Não rode o Gateway como root apenas para acessar hidraw. Depois de obter VID/PID reais, crie uma regra específica para a controladora, por exemplo:

```text
SUBSYSTEM=="hidraw", ATTRS{idVendor}=="1234", ATTRS{idProduct}=="abcd", GROUP="rc-gateway", MODE="0660"
```

Substitua os IDs de exemplo. Salve a regra em `/etc/udev/rules.d/`, recarregue as regras e reconecte o equipamento:

```bash
sudo udevadm control --reload-rules
sudo udevadm trigger
```

Depois confirme que o nó correto pode ser lido pelo serviço `rc-gateway`. Evite regra genérica que dê acesso a todos os dispositivos `hidraw` do host.

### Socket local do provider

O provider HID publica um socket `unixpacket` em `/run/rc-gateway` com modo `0660`. Um adapter local executado sob outro usuário precisa de acesso ao grupo apropriado; não relaxe o socket para world-writable.

## `unixpacket` e framing para TCP

USB HID e CAN usam `unixpacket` porque a fronteira da mensagem importa. Se um adapter compatível precisar ficar do outro lado de TCP, configure explicitamente:

```json
"packetFraming": "length32be"
```

No stream, cada mensagem é:

```text
4 bytes uint32 big-endian length | payload original
```

Esse framing **não é Modbus**. Rapid SCADA/FUXA/driver comum só deve receber esse fluxo se existir um adapter que retire/entenda o envelope e implemente o protocolo de aplicação necessário.

## Segurança de campo

- `commandPlaneEnabled=true` é rejeitado;
- CAN TX é bloqueado por padrão (`allowTransmit=false`);
- USB HID write é bloqueado por padrão (`allowWrite=false`);
- listeners de dados não-loopback exigem allowlist CIDR;
- prefira VPN/firewall de borda para equipamentos legados sem TLS;
- use TLS 1.3/mTLS quando a topologia suportar;
- chaves/certificados devem ser legíveis apenas pelo usuário/grupo necessário;
- admin nunca deve ser publicado diretamente em uma interface externa;
- não conclua que escrita industrial está autorizada apenas porque o transporte funciona.

## Soak e impairment

CI executa mini-soak automatizado. Na VM de homologação, usando o checkout do projeto:

```bash
bash scripts/run-soak.sh 86400   # 24 h
bash scripts/run-soak.sh 604800  # 7 dias
```

Adicionar `tc netem` em ambiente HIL para perda, jitter, atraso, duplicação e flapping. O mini-soak em user-space não substitui rede/hardware reais.

## Checklist HIL antes de production validated

- PUSR/USR real em reverse TCP → Gateway → Rapid/FUXA;
- dispositivo direto por IP/VPN;
- RS232 real;
- RS422 real quando aplicável;
- RS485 real, incluindo direção/half-duplex;
- USB HID real: VID/PID/serial, report descriptor, report IDs/tamanhos, leitura, escrita somente quando autorizada, unplug/replug e power-cycle;
- InteliLite 4 AMF 9 real: captura controlada da sessão do InteliConfig/consumidor para determinar protocolo de aplicação e necessidade de adapter ComAp Direct;
- permissões udev do HID sem executar o daemon como root;
- UDP real quando aplicável;
- CAN clássico físico;
- CAN-FD físico;
- queda/reconexão de 4G/VPN/MikroTik;
- power-cycle de modem/controladora;
- restart do consumidor e do Gateway;
- 24 h sem crescimento anormal de memória/FD/goroutines; alvo 7 dias;
- rollback em máquina de homologação.

Somente após esses gates físicos o estado pode mudar de **software field-test-ready** para **production validated**.

## Diagnóstico rápido

**Gateway não sobe:** execute `--check-config` e depois consulte `journalctl`.

**`healthz` responde e `readyz` não:** algum componente configurado não inicializou localmente; verifique logs, binds, sockets, HID configurado e interface CAN.

**PUSR conectado mas consumidor não comunica:** confirme allowlist, pareamento, porta do consumidor, firewall/VPN e logs. Não modifique payload para “corrigir” protocolo.

**Serial sem comunicação:** confirme `/dev/tty*`, grupo `dialout`, baud/data/parity/stop bits e direção RS485 do adaptador.

**USB HID não sobe:** execute o probe, confirme se o seletor encontra exatamente uma unidade, valide udev/permissões, confira se o nó é character device e não symlink. Se o transporte sobe mas não há leitura útil, o problema pode estar no protocolo ComAp Direct/adapter, não no hidraw.

**Erro `requires network unixpacket`:** o túnel está tentando consumir um provider packet com Unix stream. Use `network: "unixpacket"`.

**Erro exigindo `packetFraming=length32be`:** o túnel mistura packet e stream. Adicione framing somente se o consumidor remoto conhecer o envelope; não o use para fingir que HID virou Modbus.

**CAN sem comunicação:** confirme `ip -details link show`, interface UP, bitrate/FD e transceiver. J1939/CANopen são interpretados pelo consumidor, não pelo Gateway.

**Release nova falha:** o instalador executa rollback automático; confira `current`, `previous`, status systemd e logs antes de nova tentativa.
