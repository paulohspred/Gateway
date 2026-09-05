# Production Readiness Matrix — RC Universal Gateway

> Estado/handoff canônico: [`PROJECT_STATE.md`](./PROJECT_STATE.md).  
> Procedimentos: [`RUNBOOK.md`](./RUNBOOK.md).  
> Compatibilidade: [`COMPATIBILITY_MATRIX.md`](./COMPATIBILITY_MATRIX.md).

O critério de produção é **qualidade da ponte**, não quantidade de protocolos interpretados.

## Core e transportes

| Capacidade | Estado |
|---|---|
| Tunnel raw duplex | software validated |
| TCP listen/connect | software validated |
| listen ↔ listen / connect ↔ listen | sockets reais validados |
| byte-for-byte bidirecional | validado |
| pair timeout | validado |
| slow-peer/write timeout | validado |
| half-close drain | validado |
| RST/queda abrupta | validado |
| reconnect/churn | 1.000 ciclos no stress gate |
| escala/concurrency | 1.000 pares duplex simultâneos no stress gate |
| FD/goroutine leak gate | validado |
| impairment user-space | validado |
| mini-soak CI | validado |
| TLS 1.3/mTLS | software validated |
| Unix socket | software validated |
| RS232/422/485 | software validated; HIL físico pendente |
| UDP | software validated |
| SocketCAN/CAN-FD | software validated; `vcan` executa quando kernel oferece módulo; HIL físico pendente |
| métricas/sessões | implementado |
| CIDR allowlist | implementado |
| TCP keepalive/NODELAY | implementado |
| race detector | gate CI |
| Command Plane | bloqueado |

## Configuração

| Gate | Estado |
|---|---|
| JSON estrito / campos desconhecidos | validado |
| JSON extra/trailing document | validado |
| IDs únicos entre recursos | validado |
| colisão TCP/admin bind | validado |
| colisão UDP bind | validado |
| colisão Unix/provider | validado |
| porta serial duplicada | validado |
| `--check-config` sem abrir transports | validado |
| `--version` | validado |
| exemplos `configs/*.json` | validados pelo binário no CI |

## Release e supply chain

Validado no workflow `Gateway Umbrella`:

- build Linux amd64 e arm64;
- `-trimpath` e metadados de versão/commit;
- pacote tar.gz determinístico/reproduzível;
- SHA256;
- SBOM CycloneDX;
- `govulncheck` compatível com Go 1.27.1;
- dry-run do instalador contra o pacote real;
- unit systemd standalone;
- releases imutáveis com `current`/`previous`;
- configuração validada antes da troca;
- health/readiness após restart;
- rollback automático em falha;
- rollback manual com health gate;
- release candidate publicada como artifact do CI.

## Segurança

- `commandPlaneEnabled=true` é rejeitado;
- CAN TX é bloqueado por padrão;
- configuração pública deve usar allowlist/firewall/VPN conforme topologia;
- TLS/mTLS é transporte, não autorização para comandos;
- admin deve permanecer em rede local/management;
- nenhuma liberação de escrita industrial nasce do suporte ao transporte.

## Gate final automatizável do produto Gateway

**Software field-test-ready** exige, no mesmo HEAD do Gateway:

1. Canonical project state;
2. Bridge Core Go;
3. Stress and leak gate;
4. Impairment and mini-soak gate;
5. Release and supply-chain gate;
6. CI geral sem regressão causada pelo Gateway.

O workflow global `Quality and Security` do monorepo é acompanhado separadamente. Falhas comprovadamente externas a `gateway-umbrella/` — por exemplo auditoria Node do frontend legado — não invalidam a qualificação do produto Gateway, embora ainda possam bloquear merge do PR conforme a política do repositório e devam ser corrigidas no escopo correspondente. O Gateway possui seu próprio vulnerability/supply-chain gate obrigatório e não ignora vulnerabilidades Go alcançáveis.

## Gates físicos restantes

Depois dos gates automatizáveis ainda permanecem:

- PUSR/USR real;
- controlador/dispositivo real;
- VPN/4G/MikroTik real;
- serial real;
- CAN/CAN-FD físico;
- `tc netem`/falhas de rede em HIL;
- soak mínimo 24 h, alvo 7 dias;
- rollback em máquina de homologação.

Esses itens físicos são o que transforma **software field-test-ready** em **production validated**.
