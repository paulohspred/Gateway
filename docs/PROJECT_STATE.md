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

O produto agora vive diretamente na raiz de `github.com/paulohspred/Gateway`.

Não existe mais a camada de diretório `gateway-umbrella/`. O módulo Go é:

```text
github.com/paulohspred/Gateway
```

Estrutura principal:

```text
cmd/ internal/ configs/ docs/ scripts/ systemd/ .github/
```

## Estado deste ciclo

Este ciclo executa reorganização standalone e hardening antes de promover um novo checkpoint. Até o GitHub Actions do novo repositório ficar integralmente verde, o HEAD desta branch deve ser tratado como **candidate / validation pending**.

O último código de origem já havia sido validado no monorepo anterior, mas essa evidência não substitui repetir os gates depois das mudanças de segurança e lifecycle feitas aqui.

### Mudanças realizadas

- conteúdo do produto promovido para a raiz do repositório;
- module/import path migrado para `github.com/paulohspred/Gateway`;
- scripts restaurados como executáveis;
- GitHub Actions standalone restaurado;
- registry de métricas passa a tirar snapshot e libera lock antes de I/O HTTP;
- admin HTTP passa a aceitar somente loopback pela configuração de produção;
- admin recebe timeouts completos, limite de headers, GET-only e `nosniff`;
- opções TLS são fail-closed: configuração TLS com `enabled=false` é rejeitada;
- listeners TCP/UDP públicos exigem allowlist independentemente do flag legado;
- caminhos de sockets de providers são canonicalizados antes de comparação;
- IDs que colidem depois da sanitização de métricas são rejeitados;
- provider CAN não remove arquivo regular em caminho de socket e valida existência da interface no startup;
- providers serial/Unix criam diretório de socket com permissões restritas e sockets `0660`;
- expiração UDP revalida `lastSeen` e toca sessões existentes sob lock;
- runtime só marca `/readyz` após todos os componentes configurados inicializarem sua camada local;
- erro fatal de componente cancela o runtime interno e aguarda goroutines antes de retornar;
- allowlist normaliza prefixos e IPv4-mapped peer addresses;
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
- nenhum payload pode ser alterado silenciosamente;
- nenhum recurso deve crescer sem limite.

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
12. build Linux amd64/arm64 reproduzível;
13. SHA256;
14. SBOM CycloneDX;
15. dry-run real do instalador;
16. artifact de release;
17. provenance attestation em release promovida para `main`.

O checkpoint standalone só deve ser registrado abaixo depois que esses jobs terminarem com sucesso.

## Release industrial standalone

- raiz de instalação `/opt/rc-gateway-umbrella`;
- releases imutáveis em `releases/<versão>`;
- symlinks `current` e `previous`;
- systemd com `ExecStartPre --check-config`;
- Linux amd64/arm64 com `-trimpath` e metadados embutidos;
- timestamp derivado do commit e pacotes reprodutíveis;
- SHA256 + SBOM;
- configuração validada antes da troca;
- troca atômica de release;
- readiness após restart;
- rollback automático e rollback manual com health gate.

## Checkpoint standalone atual

**Pendente:** preencher com o SHA do HEAD somente depois do workflow `Gateway CI` ficar integralmente verde. Não promover documentação para “software field-test-ready” antes disso.

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
