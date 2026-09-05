# RC Universal Gateway

> Antes de alterar o runtime, leia [`docs/PROJECT_STATE.md`](./docs/PROJECT_STATE.md) e mantenha o handoff atualizado no mesmo conjunto de mudanças.

RC Universal Gateway é um **gateway industrial/IoT bridge-first**. O core transporta bytes, datagramas, frames e reports entre o campo e o software que entende o equipamento, sem introduzir banco de registradores, polling semântico ou historian.

```text
BRIDGE FIRST
PROTOCOL OPTIONAL
NO DEVICE MEMORY DATABASE
NO TELEMETRY HISTORIAN
```

## Fluxo

```text
Controladora / PLC / RTU / IED
   |
RS232 / RS422 / RS485 / Ethernet / USB HID / CAN / UDP
   |
modem / VPN / rede local
   |
RC UNIVERSAL GATEWAY
   |
ponte raw / packet-preserving
   |
Rapid SCADA / FUXA / software do fabricante / outro consumidor
```

O consumidor envia as requisições. O Gateway encaminha o payload sem interpretar registradores ou alterar silenciosamente o conteúdo.

## Transportes implementados

### Stream

- TCP `listen`/`connect`;
- TLS 1.3 e mTLS;
- Unix stream sockets;
- RS232/RS422/RS485 via provider serial raw.

Cada túnel possui `field` e `consumer`. Em `listen ↔ connect`, o peer inbound dispara a conexão para a outra ponta.

### Datagram

UDP preserva a fronteira de cada datagrama, mantém sessão isolada por peer, aplica idle timeout, allowlist e limites explícitos de sessões/tamanho.

### Frame / report / packet

- `unixpacket` (`AF_UNIX/SOCK_SEQPACKET`) é suportado pelo bridge genérico para manter fronteiras de mensagem;
- SocketCAN/CAN-FD preserva frames do ABI Linux. J1939/CANopen permanecem responsabilidade do consumidor. Transmissão CAN é **bloqueada por padrão** (`allowTransmit=false`);
- USB HID Linux preserva cada report em `unixpacket`. Escrita no dispositivo é **bloqueada por padrão** (`allowWrite=false`);
- USB HID pode usar `/dev/hidrawN` explícito ou autodiscovery estável por `vendorId` + `productId` + `serialNumber` opcional;
- ao cruzar `unixpacket` com TCP/Unix stream, o Gateway exige `packetFraming: "length32be"`; não existe perda silenciosa de fronteiras.

O provider USB HID é uma camada de transporte. Ele não converte automaticamente o protocolo proprietário ComAp Direct para Modbus. Para uma InteliLite 4 AMF 9 conectada por USB, VID/PID/serial, report descriptor, reports e protocolo de aplicação precisam ser confirmados em HIL. Consulte [`docs/USB_HID_COMAP.md`](./docs/USB_HID_COMAP.md).

## Segurança por padrão

- `commandPlaneEnabled=true` é rejeitado;
- administração HTTP é restrita a loopback nesta release;
- listeners TCP/UDP não-loopback exigem `allowedCidrs` mesmo quando configurações legadas tentam desabilitar a política;
- opções TLS são rejeitadas se `tls.enabled=false`;
- TLS listener exige chave/certificado e mTLS exige CA;
- CAN TX permanece desabilitado por padrão;
- USB HID write permanece desabilitado por padrão;
- HID exige `/dev/hidrawN` ou seletor VID/PID válido; ambiguidade de autodiscovery falha e exige serial/caminho explícito;
- caminho HID + seletor são verificados entre si antes do startup;
- HID configurado precisa existir e ser character device antes de o provider declarar readiness;
- provider sockets possuem tipo conhecido: serial usa `unix`, HID/CAN usam `unixpacket`; mismatch de configuração é rejeitado;
- transição packet↔stream exige framing explícito `length32be`;
- caminhos de sockets Unix são normalizados e arquivos comuns nunca são removidos como se fossem sockets stale;
- `LoadStrict` valida a mesma fotografia do arquivo que foi lida, evitando dupla leitura/TOCTOU da configuração;
- release archives rejeitam symlinks, hardlinks e entradas especiais;
- firewall/VPN continuam fazendo parte do plano de rede OT.

Admin padrão: `127.0.0.1:18080`.

## USB HID estável

Primeiro descubra os metadados reais do equipamento:

```bash
bash scripts/probe-usb-hid.sh
```

O probe coleta, quando disponíveis, VID/PID, serial HID/USB, fabricante/produto, interface, tamanho + SHA-256 do report descriptor e permissões do `/dev/hidrawN`.

Uma configuração de produção pode então usar identidade estável:

```json
{
  "usbHidProviders": [
    {
      "id": "controller-usb",
      "socket": "/run/rc-gateway/controller.hid.sock",
      "vendorId": "1234",
      "productId": "abcd",
      "serialNumber": "SERIAL-REAL-DO-EQUIPAMENTO",
      "maxReportBytes": 4096,
      "allowWrite": false
    }
  ]
}
```

Os IDs acima são apenas exemplo de formato; use os valores do hardware real.

## Packet framing explícito

Quando exatamente uma ponta do túnel usa `unixpacket` e a outra é TCP/Unix stream, configure:

```json
{
  "packetFraming": "length32be"
}
```

Cada pacote no stream vira:

```text
uint32 big-endian length | payload original
```

O limite é 64 KiB por frame. Esse envelope é um contrato do Gateway e **não é Modbus**; o cliente/adapter do lado stream precisa entendê-lo.

## Estrutura do repositório

```text
.
├── cmd/rc-gateway/             # entrypoint
├── internal/
│   ├── admin/                  # health/readiness/status/metrics
│   ├── bridge/                 # TCP/TLS/Unix/unixpacket/framing
│   ├── config/                 # schema e validação fail-closed
│   ├── core/                   # sessões
│   ├── datagram/               # UDP por peer
│   ├── gateway/                # orquestração do runtime
│   ├── metrics/                # métricas operacionais
│   ├── provider/               # serial, USB HID e SocketCAN
│   └── transport/netutil/      # allowlists de rede
├── configs/                    # exemplos validados pelo CI
├── docs/                       # arquitetura, runbook e matrizes
├── scripts/                    # CI local, probes, release, install e rollback
├── systemd/                    # unit endurecida
├── .github/workflows/ci.yml    # gates automatizados
├── go.mod
└── README.md
```

O código fica diretamente na raiz porque este repositório é o produto standalone; não existe mais uma pasta intermediária `gateway-umbrella/`.

## Desenvolvimento

```bash
go test ./... -shuffle=on -count=1
go test -race ./... -count=1
go vet ./...
go build -trimpath ./cmd/rc-gateway
./rc-gateway --check-config --config ./configs/gateway.example.json
```

Ou execute o gate local completo:

```bash
bash scripts/ci.sh
```

## CI e gates

O GitHub Actions executa, em sequência:

1. sincronismo do `PROJECT_STATE.md`;
2. `gofmt`, `go vet`, testes e cobertura;
3. race detector;
4. build e validação de todos os exemplos;
5. 1.000 pares duplex simultâneos + 1.000 ciclos de churn TCP;
6. impairment + mini-soak;
7. `govulncheck`;
8. testes de segurança do instalador;
9. build Linux `amd64`/`arm64` reproduzível;
10. SBOM CycloneDX;
11. SHA256, dry-run do instalador e artifact de release.

As actions de terceiros usam tags de versão principal no workflow atual, com `persist-credentials: false` no checkout. As ferramentas Go de supply chain possuem versões/commits fixados. `/readyz` só fica verde depois que todos os componentes configurados inicializam sua camada local de runtime. Isso não substitui HIL físico.

O checkpoint `1262dce7256b3fb6015ea1ccba126d460fe4be7f` teve os quatro jobs do CI verdes antes do incremento atual de descoberta HID/framing. O HEAD novo deve ficar integralmente verde novamente antes de promoção para field-test-ready.

## Release standalone

```text
/opt/rc-gateway-umbrella/
├── releases/<versão>/
├── current -> releases/<versão-atual>
└── previous -> releases/<versão-anterior>
```

Construir:

```bash
REQUIRE_SBOM=1 ARCHES="amd64 arm64" \
  bash scripts/build-release.sh <versão>
```

Pré-validar:

```bash
bash scripts/install-release.sh --dry-run \
  dist/<release>.tar.gz \
  dist/<release>.tar.gz.sha256 \
  /caminho/config.json
```

Instalar/atualizar:

```bash
sudo bash scripts/install-release.sh \
  dist/<release>.tar.gz \
  dist/<release>.tar.gz.sha256 \
  /caminho/config.json
```

Rollback:

```bash
sudo /opt/rc-gateway-umbrella/current/scripts/rollback-release.sh
```

O instalador valida integridade, estrutura e configuração antes da troca atômica; rejeita tipos de archive inseguros, limita backups de configuração, exige readiness após restart e executa rollback automático em falha.

## Documentação

- [`docs/PROJECT_STATE.md`](./docs/PROJECT_STATE.md) — estado/handoff canônico;
- [`docs/ARCHITECTURE.md`](./docs/ARCHITECTURE.md) — arquitetura;
- [`docs/PRODUCTION_MATRIX.md`](./docs/PRODUCTION_MATRIX.md) — gates de produção;
- [`docs/COMPATIBILITY_MATRIX.md`](./docs/COMPATIBILITY_MATRIX.md) — transportes/protocolos e limites;
- [`docs/USB_HID_COMAP.md`](./docs/USB_HID_COMAP.md) — provider HID, autodiscovery, framing e plano de HIL/adapter ComAp;
- [`docs/RUNBOOK.md`](./docs/RUNBOOK.md) — instalação, operação, diagnóstico, rollback e HIL;
- [`docs/PLUGIN_CONTRACT.md`](./docs/PLUGIN_CONTRACT.md) — contrato de providers.
