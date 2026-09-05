# Production Readiness Matrix — RC Universal Gateway

> Estado/handoff canônico: [`PROJECT_STATE.md`](./PROJECT_STATE.md). Procedimentos: [`RUNBOOK.md`](./RUNBOOK.md). Compatibilidade: [`COMPATIBILITY_MATRIX.md`](./COMPATIBILITY_MATRIX.md).

O critério de qualidade é a integridade e previsibilidade da ponte, não a quantidade de protocolos interpretados.

## Core e transportes

| Capacidade | Estado automatizável |
|---|---|
| Tunnel raw duplex | validado por testes |
| TCP listen/connect | validado por testes |
| listen ↔ listen / listen ↔ connect | validado por sockets/testes |
| byte-for-byte bidirecional | validado |
| pair timeout | validado |
| write timeout / slow peer | validado |
| half-close drain | validado |
| RST/queda abrupta | validado |
| reconnect/churn | gate de 1.000 ciclos |
| escala/concurrency | gate de 1.000 pares duplex |
| FD/goroutine leak | gate automatizado |
| impairment user-space | validado |
| mini-soak | gate automatizado |
| TLS 1.3/mTLS | testes + configuração fail-closed |
| Unix stream | validado |
| Unix `SOCK_SEQPACKET` | validado com preservação de mensagens nos dois sentidos |
| packet↔stream `length32be` | validado nos dois sentidos; framing obrigatório para mix packet/stream |
| RS232/422/485 | software validado; HIL físico pendente |
| USB HID `/dev/hidrawN` | transporte + framing/safety testados; HIL físico pendente |
| USB HID VID/PID/serial | autodiscovery/validação fail-closed testados; HIL físico pendente |
| UDP | software validado; sessão por peer limitada |
| SocketCAN/CAN-FD | software validado; HIL físico pendente |
| métricas/sessões | implementado/testado |
| CIDR allowlist | fail-closed para listener não-loopback |
| TCP keepalive/NODELAY | implementado |
| Command Plane | bloqueado |

## Segurança e disponibilidade

| Gate | Estado |
|---|---|
| admin HTTP loopback-only | obrigatório por configuração |
| rotas admin GET-only | implementado/testado |
| timeouts e limite de header HTTP | implementado |
| scrape de métricas sem manter lock do data plane | teste de regressão |
| configuração estrita sobre uma única fotografia de bytes | implementado; dupla leitura/TOCTOU removida |
| TLS options com TLS desabilitado | rejeitadas |
| mTLS sem CA | rejeitado |
| listener TCP/UDP público sem allowlist | rejeitado |
| IDs que colidem após sanitização de métricas | rejeitados |
| socket provider canonicalizado | validado |
| tipo de socket provider incompatível com tunnel | rejeitado antes do runtime |
| transição `unixpacket`↔stream sem framing | rejeitada |
| CAN startup removendo arquivo regular | bloqueado/testado |
| USB HID sem `device` nem VID/PID | rejeitado |
| USB HID seletor VID/PID incompleto | rejeitado |
| USB HID autodiscovery ambíguo | rejeitado; exige serial/path |
| USB HID path + seletor apontando para hardware diferente | rejeitado |
| USB HID ausente/non-character-device antes de readiness | rejeitado |
| USB HID symlink | rejeitado |
| USB HID write sem opt-in | bloqueado/testado |
| UDP idle cleanup concorrente | revalidação de `lastSeen` |
| readiness antes de componentes locais iniciarem | bloqueada por barrier de readiness |
| erro fatal deixando goroutines órfãs | runtime cancela e aguarda shutdown |

## Configuração

| Gate | Estado |
|---|---|
| JSON estrito / campos desconhecidos | validado |
| JSON extra/trailing document | validado |
| arquivo lido uma vez em `LoadStrict` | implementado |
| IDs únicos entre recursos | validado |
| colisão TCP/admin bind | validado |
| colisão UDP bind | validado |
| colisão Unix/provider | validado |
| porta serial duplicada | validado |
| dispositivo físico serial/USB duplicado | validado |
| seletor HID automático duplicado | validado |
| provider socket `unix` vs `unixpacket` | validado |
| `packetFraming` coerente com tipos de endpoint | validado |
| `--check-config` sem abrir transports | validado |
| `--version` | validado |
| exemplos `configs/*.json` | gate CI |

## Testes automatizados

O workflow standalone `Gateway CI` executa no mesmo change set:

1. sincronismo do estado canônico;
2. `gofmt` e `go vet`;
3. testes unitários/integrados com shuffle e cobertura;
4. race detector;
5. build e validação de todas as configurações de exemplo;
6. 1.000 pares duplex simultâneos;
7. 1.000 ciclos de churn TCP com leak gate;
8. impairment + mini-soak;
9. `govulncheck`;
10. shell syntax;
11. testes de archive malicioso do instalador;
12. build Linux amd64/arm64 reproduzível;
13. SHA256 e SBOM CycloneDX;
14. dry-run do instalador contra pacote real;
15. artifact de release candidate.

As actions de terceiros usam tags de versão principal no workflow atual, com checkout sem persistência de credenciais. Ferramentas Go de supply chain permanecem pinadas. Pinagem das actions por commit SHA é melhoria pendente de revalidação do workflow.

## Release e supply chain

- build Linux amd64/arm64 com `-trimpath` e metadados;
- pacote determinístico/reproduzível;
- SHA256;
- SBOM CycloneDX;
- `govulncheck` pinado/compatível com Go 1.27.1;
- archives aceitam somente diretórios e arquivos regulares, sem links/entradas especiais;
- uma única raiz de pacote é obrigatória;
- dry-run valida pacote/config sem modificar host;
- releases imutáveis com `current`/`previous`;
- backups de configuração possuem retenção limitada;
- troca de release é atômica;
- restart exige readiness;
- rollback automático em falha e rollback manual com health gate.

## Critério de promoção automatizável

**Software field-test-ready** só pode ser declarado quando todos os jobs de `Gateway CI` estiverem verdes no mesmo código/change set. `PROJECT_STATE.md` registra o checkpoint promovido.

O checkpoint `1262dce7256b3fb6015ea1ccba126d460fe4be7f` foi integralmente verde antes do incremento de identidade HID/unixpacket/framing. Mudanças posteriores precisam fechar novamente todos os gates no mesmo HEAD.

## Gates físicos restantes

Mesmo com todos os gates automatizados verdes, permanecem necessários para **production validated**:

- PUSR/USR real;
- controlador/dispositivo real;
- VPN/4G/MikroTik real;
- RS232/RS422/RS485 reais;
- USB HID real, incluindo enumeração, VID/PID/serial, report descriptor, report IDs/tamanhos, permissões udev e reconexão;
- InteliLite 4 AMF 9 real para determinar o protocolo de aplicação sobre USB e necessidade de adapter ComAp Direct;
- UDP físico quando aplicável;
- CAN/CAN-FD físico;
- `tc netem`/falhas de rede em HIL;
- power-cycle/reconnect reais;
- soak mínimo 24 h, alvo 7 dias;
- rollback em máquina de homologação.

Esses gates físicos não podem ser substituídos por CI.
