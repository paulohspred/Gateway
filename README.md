# RC Universal Gateway

> Antes de alterar o runtime, leia [`docs/PROJECT_STATE.md`](./docs/PROJECT_STATE.md) e mantenha o handoff atualizado no mesmo conjunto de mudanças.

RC Universal Gateway é um **gateway industrial/IoT bridge-first**. O core transporta bytes, datagramas e frames entre o campo e o software que entende o equipamento, sem introduzir banco de registradores, polling semântico ou historian.

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
RS232 / RS422 / RS485 / Ethernet / CAN / UDP
   |
modem / VPN / rede local
   |
RC UNIVERSAL GATEWAY
   |
ponte raw
   |
Rapid SCADA / FUXA / software do fabricante / outro consumidor
```

O consumidor envia as requisições. O Gateway encaminha o payload sem interpretar registradores ou alterar silenciosamente o conteúdo.

## Transportes implementados

### Stream

- TCP `listen`/`connect`;
- TLS 1.3 e mTLS;
- Unix sockets;
- RS232/RS422/RS485 via provider serial raw.

Cada túnel possui `field` e `consumer`. Em `listen ↔ connect`, o peer inbound dispara a conexão para a outra ponta.

### Datagram

UDP preserva a fronteira de cada datagrama, mantém sessão isolada por peer, aplica idle timeout, allowlist e limites explícitos de sessões/tamanho.

### Frame

SocketCAN/CAN-FD preserva frames do ABI Linux. J1939/CANopen permanecem responsabilidade do consumidor. Transmissão CAN é **bloqueada por padrão** (`allowTransmit=false`).

## Segurança por padrão

- `commandPlaneEnabled=true` é rejeitado;
- administração HTTP é restrita a loopback nesta release;
- listeners TCP/UDP não-loopback exigem `allowedCidrs` mesmo quando configurações legadas tentam desabilitar a política;
- opções TLS são rejeitadas se `tls.enabled=false`;
- TLS listener exige chave/certificado e mTLS exige CA;
- CAN TX permanece desabilitado por padrão;
- caminhos de sockets Unix são normalizados e arquivos comuns nunca são removidos como se fossem sockets stale;
- release archives rejeitam symlinks, hardlinks e entradas especiais;
- firewall/VPN continuam fazendo parte do plano de rede OT.

Admin padrão: `127.0.0.1:18080`.

## Estrutura do repositório

```text
.
├── cmd/rc-gateway/             # entrypoint
├── internal/
│   ├── admin/                  # health/readiness/status/metrics
│   ├── bridge/                 # stream duplex/TLS/Unix
│   ├── config/                 # schema e validação fail-closed
│   ├── core/                   # sessões
│   ├── datagram/               # UDP por peer
│   ├── gateway/                # orquestração do runtime
│   ├── metrics/                # métricas operacionais
│   ├── provider/               # serial e SocketCAN
│   └── transport/netutil/      # allowlists de rede
├── configs/                    # exemplos validados pelo CI
├── docs/                       # arquitetura, runbook e matrizes
├── scripts/                    # CI local, release, install e rollback
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

As GitHub Actions usadas no workflow são fixadas por commit SHA. `/readyz` só fica verde depois que todos os componentes configurados inicializam sua camada local de runtime. Isso não substitui HIL físico: serial/CAN/dispositivo remoto ainda precisam de homologação real.

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
- [`docs/RUNBOOK.md`](./docs/RUNBOOK.md) — instalação, operação, diagnóstico, rollback e HIL;
- [`docs/PLUGIN_CONTRACT.md`](./docs/PLUGIN_CONTRACT.md) — contrato de providers.
