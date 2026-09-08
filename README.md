# RC Universal Gateway

> Estado técnico canônico: [`docs/PROJECT_STATE.md`](./docs/PROJECT_STATE.md). Procedimentos de campo: [`docs/RUNBOOK.md`](./docs/RUNBOOK.md).

RC Universal Gateway é um **gateway industrial/IoT bridge-first**. O core transporta bytes, datagramas, frames e reports entre equipamentos de campo e o software que entende o protocolo de aplicação, sem introduzir banco de registradores, polling semântico, historian ou conversão silenciosa.

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

O consumidor envia as requisições. O Gateway encaminha o payload sem interpretar registradores nem alterar silenciosamente o conteúdo.

## Transportes implementados

### Stream

- TCP `listen`/`connect`;
- reverse TCP de modem;
- TCP direto por IP/VPN;
- TLS 1.3 e mTLS;
- Unix stream sockets;
- RS232/RS422/RS485 via provider serial raw.

Túneis possuem limites de pares simultâneos por tunnel e um limite global. Paralelismo maior é permitido somente quando a topologia de pareamento é inequívoca; configurações ambíguas são rejeitadas.

### Datagram

UDP preserva a fronteira de cada datagrama, mantém sessão isolada por peer, aplica idle timeout, allowlist e limites explícitos de sessões/tamanho.

### Frame / report / packet

- `unixpacket` (`AF_UNIX/SOCK_SEQPACKET`) mantém fronteiras de mensagem;
- SocketCAN/CAN-FD preserva frames do ABI Linux. J1939/CANopen permanecem responsabilidade do consumidor. Transmissão CAN é **bloqueada por padrão** (`allowTransmit=false`);
- USB HID Linux preserva cada report em `unixpacket`. Escrita é **bloqueada por padrão** (`allowWrite=false`);
- USB HID aceita `/dev/hidrawN` explícito ou autodiscovery por `vendorId` + `productId` + `serialNumber` opcional;
- ao cruzar `unixpacket` com TCP/Unix stream, o Gateway exige `packetFraming: "length32be"` para não perder fronteiras silenciosamente.

O provider USB HID é transporte. Ele não converte automaticamente ComAp Direct para Modbus. Para InteliLite 4 AMF 9 por USB, VID/PID/serial, descriptor HID, tamanhos/report IDs e protocolo de aplicação continuam dependentes de HIL. Consulte [`docs/USB_HID_COMAP.md`](./docs/USB_HID_COMAP.md).

## RS485 multidrop

O Gateway transporta o barramento serial de forma transparente. Várias controladoras podem compartilhar o mesmo RS485 quando a própria rede serial/protocolo suporta multidrop: por exemplo, Modbus RTU com IDs distintos, mesmos parâmetros seriais e cabeamento/terminação corretos.

Modelos diferentes podem coexistir desde que sejam eletricamente e protocolarmente compatíveis no mesmo barramento. O mapa de registradores e a semântica de cada modelo pertencem ao Rapid SCADA/FUXA/driver, não ao Gateway.

## Integração nativa com Rapid SCADA v6

O repositório inclui agora um contrato de integração específico para Rapid SCADA v6, revisado contra `RapidScada/scada-v6` `master` commit `1fd36080c7830303f921672fdaee335a06e7ae50`.

A integração **não copia e não incorpora Rapid SCADA**. Ela usa os componentes nativos do Rapid SCADA:

```text
DrvCnlBasic / TcpClient
        |
        v
consumer TCP do RC Gateway
        |
        v
campo / modem / serial / VPN
```

Para Modbus, o Rapid SCADA `DrvModbus` deve usar o modo correspondente aos bytes entregues pelo Gateway:

```text
Modbus TCP nativo      -> TransMode=TCP
Modbus RTU raw em TCP  -> TransMode=RTU
Modbus ASCII em TCP    -> TransMode=ASCII
```

Para um único RS485 multidrop, a recomendação é `ConnectionMode=Shared` no Rapid SCADA e vários dispositivos/IDs na mesma communication line. Isso preserva um único mestre efetivo no barramento.

Perfis prontos:

- `configs/rapid-scada.modbus-tcp.example.json`;
- `configs/rapid-scada.rtu-over-tcp.example.json`;
- `configs/rapid-scada.rs485-multidrop.example.json`.

Procedimento completo: [`docs/RAPID_SCADA_INTEGRATION.md`](./docs/RAPID_SCADA_INTEGRATION.md).

Depois de instalar/configurar o Rapid SCADA real na VM:

```bash
sudo RAPID_SCADA_COMM_SERVICE=scadacomm6.service \
  /opt/rc-gateway/current/scripts/rapid-scada-acceptance.sh
```

Esse teste exige uma sessão real do Rapid SCADA Communicator através do Gateway antes de promover a combinação para `vm_accepted`.

## Segurança por padrão

- `commandPlaneEnabled=true` é rejeitado;
- admin HTTP é loopback-only nesta release;
- listeners TCP/UDP não-loopback exigem `allowedCidrs`;
- opções TLS são rejeitadas se `tls.enabled=false`;
- listener TLS exige chave/certificado e mTLS exige CA;
- CAN TX e USB HID write ficam desabilitados por padrão;
- HID ambíguo, symlink, non-character-device e path/identidade divergentes são rejeitados;
- serial usa provider socket `unix`; HID/CAN usam `unixpacket`;
- packet↔stream exige framing explícito;
- sockets/paths são canonicalizados e arquivos comuns não são removidos como stale sockets;
- `LoadStrict` valida uma única fotografia do arquivo de configuração;
- pares stream e sessões UDP são limitados;
- release archives rejeitam traversal, links e entradas especiais;
- instalação valida config antes da ativação e faz rollback se readiness falhar.

Admin padrão: `127.0.0.1:18080`.

## Observabilidade

Endpoints locais GET-only:

```text
/healthz   /v1/healthz
/readyz    /v1/readyz
/status    /v1/status
/sessions  /v1/sessions
/metrics   /v1/metrics
```

`/readyz` prova inicialização local dos componentes configurados. Não prova que uma controladora respondeu ao protocolo de aplicação.

O serviço systemd usa `Type=notify`, readiness e watchdog.

## USB HID estável

Descubra os metadados reais:

```bash
bash scripts/probe-usb-hid.sh
```

O probe coleta, quando disponíveis, VID/PID, serial HID/USB, fabricante/produto, interface, tamanho + SHA-256 do report descriptor e permissões do `/dev/hidrawN`.

Exemplo de seletor estável:

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

Os IDs são apenas exemplo de formato; use valores medidos no hardware.

## Packet framing explícito

Quando exatamente uma ponta usa `unixpacket` e a outra é stream:

```json
{
  "packetFraming": "length32be"
}
```

Cada pacote no stream vira:

```text
uint32 big-endian length | payload original
```

Esse envelope é um contrato do Gateway e **não é Modbus**. O cliente/adapter do lado stream precisa entendê-lo.

## Desenvolvimento e validação

Gate local principal:

```bash
bash scripts/ci.sh
```

Quando `actionlint` e `staticcheck` estão instalados, o script também executa esses gates. No CI eles são obrigatórios e pinados.

O pipeline do GitHub executa no mesmo candidato:

1. consistência do estado canônico;
2. `actionlint`;
3. `gofmt`;
4. `go mod verify` + tidy-diff;
5. `go vet`;
6. Staticcheck;
7. testes unitários/integrados com shuffle e cobertura mínima;
8. race detector;
9. build + validação de todos `configs/*.json`;
10. concorrência real, 1.000 pares e 1.000 churns TCP;
11. impairment + mini-soak;
12. `govulncheck`;
13. CodeQL;
14. testes de segurança do instalador;
15. build Linux amd64/arm64 reproduzível;
16. SHA256 + SBOM CycloneDX;
17. dry-run do instalador e validação do artifact.

As GitHub Actions de terceiros são pinadas por commit SHA imutável. Dependabot propõe atualizações, mas nenhuma atualização entra sem revalidar os gates.

## Release standalone

Layout canônico:

```text
/opt/rc-gateway/
├── releases/<versão>/
├── current -> releases/<versão-atual>
└── previous -> releases/<versão-anterior>

/etc/rc-gateway.json
/etc/systemd/system/rc-gateway.service
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
sudo /opt/rc-gateway/current/scripts/rollback-release.sh
```

O release leva `LICENSE`, `NOTICE`, `THIRD_PARTY_NOTICES.md`, SBOM, documentação, scripts de diagnóstico/VM/Rapid SCADA, manifest e metadados. Version tags possuem workflow dedicado de provenance/attestation.

## Licenciamento

**RC Universal Gateway é software proprietário — All Rights Reserved. Não é open source.**

A visibilidade pública do repositório não concede permissão para usar, executar, copiar para reutilização, modificar, redistribuir, implantar, criar derivados ou usar o código para treinamento de modelos. Consulte [`LICENSE`](./LICENSE) e [`NOTICE`](./NOTICE).

Componentes de terceiros mantêm suas próprias licenças. Os avisos necessários estão em [`THIRD_PARTY_NOTICES.md`](./THIRD_PARTY_NOTICES.md) e no SBOM.

Rapid SCADA é um produto externo e não é incluído no artifact do Gateway. A integração usa apenas seus canais/drivers documentados.

Se o requisito for impedir também o **acesso** ao código-fonte, e não apenas proibir seu uso por licença, o repositório precisa ser tornado privado nas configurações do GitHub.

## Documentação

- [`docs/PROJECT_STATE.md`](./docs/PROJECT_STATE.md) — estado/handoff canônico;
- [`docs/ARCHITECTURE.md`](./docs/ARCHITECTURE.md) — arquitetura;
- [`docs/PRODUCTION_MATRIX.md`](./docs/PRODUCTION_MATRIX.md) — gates de produção;
- [`docs/COMPATIBILITY_MATRIX.md`](./docs/COMPATIBILITY_MATRIX.md) — transportes/protocolos e limites;
- [`docs/RAPID_SCADA_INTEGRATION.md`](./docs/RAPID_SCADA_INTEGRATION.md) — integração nativa Rapid SCADA v6;
- [`docs/CONFIGURATION_COMPATIBILITY.md`](./docs/CONFIGURATION_COMPATIBILITY.md) — política de schema/migração;
- [`docs/USB_HID_COMAP.md`](./docs/USB_HID_COMAP.md) — HID/ComAp/HIL;
- [`docs/RUNBOOK.md`](./docs/RUNBOOK.md) — instalação/operação/rollback;
- [`docs/VM_ACCEPTANCE.md`](./docs/VM_ACCEPTANCE.md) — homologação em VM;
- [`docs/THREAT_MODEL.md`](./docs/THREAT_MODEL.md) — threat model;
- [`docs/GITHUB_PROTECTION.md`](./docs/GITHUB_PROTECTION.md) — proteção administrativa requerida;
- [`docs/PROFESSIONALIZATION_PLAN.md`](./docs/PROFESSIONALIZATION_PLAN.md) — regra de promoção;
- [`docs/PLUGIN_CONTRACT.md`](./docs/PLUGIN_CONTRACT.md) — contrato de providers.
