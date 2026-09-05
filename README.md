# RC Universal Gateway — Gateway Umbrella

> **Antes de alterar este projeto:** leia [`docs/PROJECT_STATE.md`](./docs/PROJECT_STATE.md). O handoff é obrigatório e deve ser atualizado em toda mudança.

`gateway-umbrella/` é um **gateway universal de conectividade industrial/IoT**. Seu core é uma ponte byte-transparent entre o campo e o software que realmente entende o equipamento.

## Regra central

```text
BRIDGE FIRST
PROTOCOL OPTIONAL
NO DEVICE MEMORY DATABASE
NO TELEMETRY HISTORIAN
```

O core não contém mapas de memória de ComAp, DSE, PLC, IHM ou qualquer fabricante. Não faz polling de registradores, não converte RPM/tensão/alarmes e não mantém histórico de telemetria.

## Fluxo principal

```text
Controladora / PLC / RTU / IED
   |
RS232 / RS422 / RS485 / Ethernet / CAN / UDP
   |
modem / VPN / rede local
   |
RC UNIVERSAL GATEWAY
   |
ponte raw
   |
Rapid SCADA / FUXA / software do fabricante / outro destino
```

O destino envia as requisições. O Gateway encaminha bytes/datagramas/frames até o equipamento e devolve a resposta sem interpretar registradores ou alterar o payload.

## Runtime

Schema `3`.

### Stream

- TCP `listen`/`connect`;
- TLS 1.3/mTLS;
- Unix sockets;
- RS232/RS422/RS485 via provider serial raw.

Cada túnel tem `field` e `consumer`. Quando um lado é `listen` e o outro `connect`, o inbound é o trigger: o Gateway só disca o outro extremo quando existe peer.

### Datagram

UDP preserva o limite de cada datagrama e mantém sessão isolada por peer, com idle timeout e limites explícitos.

### Frame

SocketCAN/CAN-FD preserva frames do ABI Linux. J1939/CANopen continuam sendo responsabilidade do consumidor. Transmissão CAN é bloqueada por padrão.

## Exemplos

### PUSR reverso + Rapid

```text
PUSR ----TCP----> :15003  Gateway  :25003 <----TCP---- Rapid
```

### Equipamento direto por IP/VPN

```text
10.60.20.222:502 <---- Gateway :25020 <---- Rapid
```

O Gateway não precisa saber se o endpoint é DSE, ComAp, Siemens, Schneider ou protocolo proprietário.

## Sem fan-out raw cego

Um túnel request/response tem **um consumidor ativo por vez**. Replicar bytes simultaneamente para vários mestres pode misturar transações. Fan-out de telemetria pertence ao SCADA/driver/broker ou a um componente protocol-aware com arbitragem explícita.

## Segurança

- `commandPlaneEnabled=true` é rejeitado;
- CAN TX fica `allowTransmit=false` por padrão;
- listeners públicos suportam allowlist CIDR;
- TLS/mTLS é transporte seguro, não autorização automática para comando industrial;
- admin deve permanecer em rede local/management;
- firewall/VPN continuam parte do plano de rede.

## Desenvolvimento

```bash
cd gateway-umbrella
go test ./...
go vet ./...
go build ./cmd/rc-gateway
./rc-gateway --check-config --config ./configs/gateway.example.json
```

Admin padrão: `127.0.0.1:18080`.

## Release standalone

A instalação de produção é independente do sistema RC Geradores:

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

Pré-validar sem modificar o host:

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

O instalador valida SHA256, estrutura do pacote, `--check-config`, faz troca atômica, exige readiness e restaura release/configuração anteriores automaticamente se a atualização falhar.

## Documentos

- [`docs/PROJECT_STATE.md`](./docs/PROJECT_STATE.md) — handoff canônico;
- [`docs/ARCHITECTURE.md`](./docs/ARCHITECTURE.md) — arquitetura;
- [`docs/PRODUCTION_MATRIX.md`](./docs/PRODUCTION_MATRIX.md) — gates de produção;
- [`docs/COMPATIBILITY_MATRIX.md`](./docs/COMPATIBILITY_MATRIX.md) — meios/protocolos transportáveis e limites;
- [`docs/RUNBOOK.md`](./docs/RUNBOOK.md) — instalação, operação, diagnóstico, rollback e HIL;
- [`docs/THINGSBOARD_REFERENCE.md`](./docs/THINGSBOARD_REFERENCE.md) — referência comparativa;
- [`docs/PLUGIN_CONTRACT.md`](./docs/PLUGIN_CONTRACT.md) — contrato de endpoint providers.
