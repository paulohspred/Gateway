# RC Universal Gateway — estado do projeto e handoff obrigatório

> **LEIA ESTE ARQUIVO PRIMEIRO antes de modificar `gateway-umbrella/`.** Toda alteração no Gateway deve atualizar este arquivo no mesmo ciclo.

## Decisão fixa

```text
BRIDGE FIRST
PROTOCOL OPTIONAL
NO DEVICE MEMORY DATABASE
NO TELEMETRY HISTORIAN
```

O Gateway é uma ponte universal de conectividade. Rapid SCADA, FUXA, ThingsBoard, software do fabricante ou outro driver interpreta registradores e protocolos de aplicação.

## Status atual

O RC Universal Gateway atingiu o estado **software field-test-ready** nos gates automatizáveis do produto.

Checkpoint de runtime/release validado: `ecb9ee0cfd8c3a3c96125230cd07a4bbd8d20987`.

Nesse checkpoint, o workflow `Gateway Umbrella` passou integralmente:

- Canonical project state;
- Bridge Core Go: format, vet, testes, race detector, build e configuração de produção;
- Stress and leak gate: 1.000 pares duplex simultâneos + 1.000 ciclos TCP churn;
- Impairment and mini-soak gate;
- Release and supply-chain gate: vulnerability scan, shell syntax, build reproduzível, SHA256, SBOM, dry-run do instalador e artifact de release.

Também passaram no mesmo checkpoint:

- CI geral;
- Quality and Security;
- Gateway Source Bundle.

O artifact de release candidate foi gerado pelo workflow para o mesmo HEAD. Este documento apenas registra o fechamento do estado validado; qualquer alteração futura de runtime/release deve repetir os gates antes de promover novo checkpoint.

## Checkpoints verdes

- `249a7f0d55c840e5e95764468a6400db8a401fea`: limpeza bridge-first.
- `9dc17491e370a59926d9069c898c0e3bba8b8171`: hardening TCP.
- `52b2d76665fb73ac212e5cf085551aa7c658c2e1`: TLS/mTLS + Unix + RST/half-close.
- `ffa2d548fb14899aad4052cc17dbe1c9d53dab92`: Serial RS232/RS422/RS485.
- `905f82c7036bb00c7539c26ce12ad0f55db5ba48`: UDP datagram/session bridge.
- `0016e2a629e2169024bfea8fd1fb66d7ec0fe1f4`: SocketCAN/CAN-FD software checkpoint.
- `5aa5eb721c76d611f25aac8d3479b336e7475ce4`: stress/leak; 1.000 pares + 1.000 churn.
- `5dc90212cb6f72138b25e51aedf30a6dcf5f150f`: impairment + mini-soak.
- `a81db7e9cce5db4f4c3107b9ff7ec76ca76678db`: configuração estrita/`--check-config`.
- `069a5d786d83e621e74328488e882d8c49165594`: release/supply-chain completo com scanner compatível com Go 1.27.1.
- `ecb9ee0cfd8c3a3c96125230cd07a4bbd8d20987`: checkpoint integrado atual; Gateway Umbrella, CI, Quality/Security e Source Bundle verdes.

## Transportes validados em software

- TCP listen/connect;
- reverse TCP de modem;
- TCP direto por IP/VPN;
- TLS 1.3 e mTLS;
- Unix sockets;
- Serial RS232/RS422/RS485 raw;
- UDP preservando datagramas e sessões por peer;
- SocketCAN/CAN-FD preservando frames do ABI Linux;
- CAN TX bloqueado por padrão (`allowTransmit=false`);
- pair timeout, slow-peer/write timeout, half-close drain, keepalive, NODELAY e CIDR allowlist;
- métricas/sessões por transporte e direção;
- churn/reconnect, RST, half-close, concorrência, leak, impairment e mini-soak automatizados.

## Configuração de produção

- JSON estrito: campos desconhecidos e documentos extras são rejeitados;
- IDs únicos entre providers/túneis;
- colisões TCP/admin e UDP detectadas antes do runtime;
- colisão Unix/provider rejeitada;
- porta serial física duplicada rejeitada;
- no máximo um túnel consumidor por socket de provider físico;
- `--check-config` valida sem abrir transports;
- `--version` expõe versão/commit/build;
- exemplos `configs/*.json` são validados no CI.

## Release industrial standalone

- raiz `/opt/rc-gateway-umbrella`;
- releases imutáveis em `releases/<versão>`;
- symlinks `current` e `previous`;
- systemd com `ExecStartPre --check-config`;
- build Linux amd64/arm64 com `-trimpath` e metadados embutidos;
- timestamp derivado do commit e pacotes reprodutíveis;
- SHA256;
- SBOM CycloneDX;
- `govulncheck` pinado na revisão upstream `8fcedea455d953a0f8470e1f41420bb6f2e72665`, compatível com Go 1.27.1;
- instalador transacional com checksum, proteção contra path traversal, staging e validação de config;
- troca atômica de release;
- readiness após restart;
- rollback automático em falha;
- rollback manual com health gate e autorreversão;
- dry-run contra pacote real no CI;
- comparação byte-a-byte de duas builds idênticas;
- artifact de release candidate no CI.

## Documentação operacional final

- `docs/RUNBOOK.md`: instalação, atualização, rollback, observabilidade, segurança, soak, HIL e diagnóstico;
- `docs/COMPATIBILITY_MATRIX.md`: meios/protocolos e limites de suporte;
- `docs/PRODUCTION_MATRIX.md`: gates automatizáveis e físicos;
- `docs/ARCHITECTURE.md`: arquitetura bridge-first;
- `docs/PROJECT_STATE.md`: handoff canônico obrigatório.

## O que ainda falta para “production validated”

Não falta desenvolvimento de software obrigatório para iniciar homologação em campo. Restam gates físicos/HIL:

1. PUSR/USR real em reverse TCP → Gateway → Rapid/FUXA/consumidor;
2. dispositivo direto por IP/VPN;
3. RS232/RS422/RS485 reais;
4. UDP real quando aplicável;
5. CAN e CAN-FD físicos;
6. VPN/4G/MikroTik e power-cycle/reconnect reais;
7. `tc netem`/impairment de rede em ambiente HIL;
8. soak mínimo 24 h, alvo 7 dias;
9. rollback em máquina de homologação.

`scripts/run-soak.sh` aceita de 1 segundo a 604800 segundos (7 dias).

## Regra de produção

- **software field-test-ready** = todos os gates automatizáveis do produto, release/supply-chain e documentação verdes;
- **production validated** = somente após HIL/soak físico da topologia real.

Não reintroduzir polling, mapas de memória ou historian no core. Nenhum payload pode ser alterado silenciosamente e nenhum recurso pode crescer sem limite. Suporte a transporte nunca libera automaticamente o Command Plane.
