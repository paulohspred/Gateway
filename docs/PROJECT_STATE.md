# RC Universal Gateway — estado do projeto e handoff

> Leia este arquivo antes de alterar runtime, segurança, release ou contratos de transporte. Mudanças técnicas relevantes devem atualizar este documento no mesmo PR/conjunto de mudanças.

## Decisão fixa

```text
BRIDGE FIRST
PROTOCOL OPTIONAL
NO DEVICE MEMORY DATABASE
NO TELEMETRY HISTORIAN
```

O Gateway é uma ponte universal de conectividade. Rapid SCADA, FUXA, software do fabricante ou outro driver interpreta registradores e protocolos de aplicação.

## Repositório standalone

O produto vive diretamente na raiz de `github.com/paulohspred/Gateway`.

Não existe mais a camada de diretório `gateway-umbrella/`. O módulo Go é:

```text
github.com/paulohspred/Gateway
```

Estrutura principal:

```text
cmd/ internal/ configs/ docs/ scripts/ systemd/ .github/
```

## Estado deste ciclo

Este ciclo reorganiza o produto como repositório standalone e aplica hardening de segurança, disponibilidade, testes e release. Enquanto o workflow `Gateway CI` do HEAD não estiver integralmente verde, o estado é **candidate / validation pending**.

O código de origem já possuía checkpoints verdes no monorepo anterior, mas toda alteração deste ciclo é revalidada no novo repositório.

### Mudanças realizadas

- conteúdo do produto promovido para a raiz do repositório;
- module/import path migrado para `github.com/paulohspred/Gateway`;
- scripts restaurados como executáveis;
- GitHub Actions standalone restaurado e actions de terceiros fixadas por commit SHA;
- registry de métricas passa a tirar snapshot e liberar lock antes de I/O HTTP;
- admin HTTP passa a aceitar somente loopback pela configuração de produção;
- admin recebe timeouts completos, limite de headers, rotas GET-only e `nosniff`;
- opções TLS são fail-closed: configuração TLS com `enabled=false` é rejeitada;
- listeners TCP/UDP públicos exigem allowlist independentemente do flag legado;
- caminhos de sockets de providers são canonicalizados antes de comparação;
- IDs que colidem depois da sanitização de métricas são rejeitados;
- provider CAN não remove arquivo regular em caminho de socket e valida existência da interface no startup;
- providers serial/Unix criam diretório de socket com permissões restritas e sockets `0660`;
- expiração UDP revalida `lastSeen` antes de remover sessão;
- runtime só marca `/readyz` depois da inicialização local de todos os componentes configurados;
- erro fatal de componente cancela o runtime interno e aguarda goroutines antes de retornar;
- allowlist normaliza prefixos e IPv4-mapped peer addresses;
- testes dedicados cobrem admin HTTP, lifecycle do admin, SessionRegistry, lifecycle/readiness do Gateway, lock de métricas, configuração fail-closed, allowlist e segurança de socket CAN;
- instalador rejeita symlinks, hardlinks e entradas especiais no archive, exige uma única raiz e limita retenção de backups de configuração;
- CI testa archives maliciosos do instalador;
- documentação e catálogo foram alinhados ao estado bridge-first atual.

## Invariantes de segurança

- `commandPlaneEnabled=true` é rejeitado;
- CAN TX permanece `allowTransmit=false` por padrão;
- admin HTTP é loopback-only nesta release;
- listener TCP/UDP não-loopback sem `allowedCidrs` é inválido;
- TLS listener exige chave/certificado;
- mTLS listener exige CA;
- opções TLS não podem ficar silenciosamente configuradas com TLS desligado;
- Unix/provider socket nunca pode sobrescrever arquivo comum;
- IDs e recursos físicos não podem colidir silenciosamente;
- release archive não pode conter links ou entradas especiais;
- nenhum payload pode ser alterado silenciosamente;
- recursos internos configuráveis possuem limites explícitos ou retenção limitada.

## Transportes implementados

- TCP listen/connect;
- reverse TCP de modem;
- TCP direto por IP/VPN;
- TLS 1.3 e mTLS;
- Unix sockets;
- serial RS232/RS422/RS485 raw;
- UDP preservando datagramas e sessões por peer;
- SocketCAN/CAN-FD preservando frames do ABI Linux;
- pair timeout, write timeout, half-close drain, keepalive, NODELAY e CIDR allowlist;
- métricas e sessões por transporte/direção.

## Semântica de readiness

`/readyz` só fica verde quando:

- admin HTTP fez bind;
- cada stream tunnel criou suas sources/listeners;
- cada UDP tunnel fez bind e resolveu target;
- cada serial provider publicou seu socket local;
- cada CAN provider encontrou a interface configurada e publicou seu socket local.

Readiness **não** significa que o equipamento físico respondeu. Serial é aberto quando uma sessão efetivamente chega e CAN raw é aberto quando um consumidor conecta. HIL continua obrigatório.

## Gates automatizados do novo repositório

`.github/workflows/ci.yml` deve ficar verde no mesmo HEAD para promover a branch:

1. handoff/documentação;
2. `gofmt`;
3. `go vet`;
4. testes unitários/integrados com shuffle e cobertura;
5. race detector;
6. build e validação de todos `configs/*.json`;
7. 1.000 pares duplex simultâneos;
8. 1.000 ciclos TCP churn + leak gate;
9. impairment + mini-soak;
10. `govulncheck`;
11. shell syntax;
12. testes de segurança do instalador;
13. build Linux amd64/arm64 reproduzível;
14. SHA256;
15. SBOM CycloneDX;
16. dry-run real do instalador;
17. artifact de release.

Dependências de GitHub Actions são referenciadas por commit SHA, e as ferramentas Go de supply chain também são pinadas.

## Release industrial standalone

- raiz de instalação `/opt/rc-gateway-umbrella`;
- releases imutáveis em `releases/<versão>`;
- symlinks `current` e `previous`;
- systemd com `ExecStartPre --check-config`;
- Linux amd64/arm64 com `-trimpath` e metadados embutidos;
- timestamp derivado do commit e pacotes reprodutíveis;
- SHA256 + SBOM;
- archive restrito a arquivos regulares/diretórios e uma única raiz;
- configuração validada antes da troca;
- backups de configuração com retenção limitada;
- troca atômica de release;
- readiness após restart;
- rollback automático e rollback manual com health gate.

## Checkpoint standalone atual

**Pendente:** registrar o SHA de código depois que `Gateway CI` ficar integralmente verde com estes gates. Não promover documentação para “software field-test-ready” antes disso.

## Gates físicos restantes para production validated

1. PUSR/USR real em reverse TCP → Gateway → Rapid/FUXA/consumidor;
2. dispositivo direto por IP/VPN;
3. RS232 real;
4. RS422 real quando aplicável;
5. RS485 real, incluindo direção/half-duplex do hardware;
6. UDP real quando aplicável;
7. CAN clássico físico;
8. CAN-FD físico;
9. VPN/4G/MikroTik real;
10. power-cycle/reconnect de modem/controladora;
11. `tc netem`/impairment em HIL;
12. soak mínimo de 24 h, alvo de 7 dias;
13. rollback em máquina de homologação.

`scripts/run-soak.sh` aceita de 1 a 604800 segundos.

## Regra de promoção

- **validation pending**: código mudou e os novos gates ainda não fecharam;
- **software field-test-ready**: todos os gates automatizados/release/supply-chain do mesmo HEAD estão verdes;
- **production validated**: somente após HIL e soak físico da topologia real.

Não reintroduzir polling, mapas de memória, historian ou command plane genérico no core.
