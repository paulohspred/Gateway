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

Este ciclo reorganiza o produto como repositório standalone e aplica hardening de segurança, disponibilidade, testes e release. O incremento USB HID agora evolui de um caminho fixo `/dev/hidrawN` para identificação estável por VID/PID/serial, re-resolução após reenumeração, validação de tipo de socket e framing explícito ao cruzar transporte packet↔stream.

O checkpoint `1262dce7256b3fb6015ea1ccba126d460fe4be7f` teve todos os jobs do `Gateway CI` verdes. Como o runtime mudou depois desse checkpoint, o HEAD atual volta ao estado **candidate / validation pending** até o workflow do mesmo HEAD fechar integralmente verde.

### Mudanças realizadas

- conteúdo do produto promovido para a raiz do repositório;
- module/import path migrado para `github.com/paulohspred/Gateway`;
- scripts restaurados como executáveis;
- GitHub Actions standalone restaurado; checkout não persiste credenciais e ferramentas Go de supply chain permanecem pinadas;
- registry de métricas passa a tirar snapshot e liberar lock antes de I/O HTTP;
- admin HTTP passa a aceitar somente loopback pela configuração de produção;
- admin recebe timeouts completos, limite de headers, rotas GET-only e `nosniff`;
- opções TLS são fail-closed: configuração TLS com `enabled=false` é rejeitada;
- listeners TCP/UDP públicos exigem allowlist independentemente do flag legado;
- caminhos de sockets de providers são canonicalizados antes de comparação;
- IDs que colidem depois da sanitização de métricas são rejeitados;
- `LoadStrict` valida e normaliza a mesma fotografia de bytes lida do arquivo, removendo a dupla leitura/TOCTOU de configuração;
- provider CAN não remove arquivo regular em caminho de socket e valida existência da interface no startup;
- providers serial/Unix criam diretório de socket com permissões restritas e sockets `0660`;
- provider USB HID Linux publica `unixpacket`, preserva reports, rejeita symlink/arquivo não-character-device e mantém escrita desabilitada por padrão;
- USB HID aceita caminho explícito `/dev/hidrawN` ou seletor estável `vendorId` + `productId` + `serialNumber` opcional;
- seleção HID automática exige exatamente um match em sysfs; ambiguidades falham e exigem serial/caminho explícito;
- seletores HID sobrepostos são rejeitados: um seletor VID/PID sem serial não pode coexistir com outro provider que reivindique uma unidade do mesmo VID/PID;
- dois providers com o mesmo VID/PID só podem coexistir por seletores de serial distintos;
- quando caminho e seletor são configurados juntos, a identidade do equipamento é verificada antes do startup;
- USB HID resolve e valida a presença do character device antes de declarar readiness;
- quando configurado por identidade estável, o HID é re-resolvido a cada nova sessão para sobreviver a unplug/replug que altere o número `hidrawN`;
- generic bridge suporta `network: "unixpacket"` para consumir corretamente providers HID/CAN;
- packet↔packet usa uma escrita por mensagem em cada sentido, sem passar pelo write-all de stream;
- configuração de produção conhece o tipo real dos provider sockets (`unix` para serial, `unixpacket` para HID/CAN) e rejeita mismatch antes do runtime;
- mistura de `unixpacket` com TCP/Unix stream é fail-closed sem `packetFraming: "length32be"`;
- `length32be` codifica cada pacote como comprimento uint32 big-endian + payload sem alteração, com limite de 64 KiB por frame;
- runtime propaga `packetFraming` e seletores HID até os componentes responsáveis;
- configuração detecta colisão de ID/socket/dispositivo físico e conflitos de seletor HID;
- runtime inclui USB HID na barrier de readiness e nas métricas/sessões operacionais;
- `scripts/probe-usb-hid.sh` coleta VID/PID, nome/serial HID, metadados USB pai, interface, report descriptor (tamanho + SHA-256) e permissões;
- artifacts de release passam a levar o probe USB e documentação operacional de HID/compatibilidade/runbook, e o CI valida a presença desses arquivos;
- o projeto externo `embyt/hid2tcp` foi usado apenas como referência conceitual; nenhum código GPL foi incorporado;
- expiração UDP revalida `lastSeen` antes de remover sessão;
- runtime só marca `/readyz` depois da inicialização local de todos os componentes configurados;
- erro fatal de componente cancela o runtime interno e aguarda goroutines antes de retornar;
- allowlist normaliza prefixos e IPv4-mapped peer addresses;
- testes dedicados cobrem admin HTTP, lifecycle do admin, SessionRegistry, lifecycle/readiness do Gateway, lock de métricas, configuração fail-closed, allowlist, segurança de socket CAN, seletores HID, overlap de seletores, descoberta/reenumeração hidraw, unixpacket, atomicidade packet↔packet e framing packet↔stream;
- instalador rejeita symlinks, hardlinks e entradas especiais no archive, exige uma única raiz e limita retenção de backups de configuração;
- CI testa archives maliciosos do instalador;
- documentação e catálogo foram alinhados ao estado bridge-first atual.

## Invariantes de segurança

- `commandPlaneEnabled=true` é rejeitado;
- CAN TX permanece `allowTransmit=false` por padrão;
- USB HID write permanece `allowWrite=false` por padrão;
- USB HID requer caminho `/dev/hidrawN` ou seletor VID/PID válido;
- seletor VID/PID sem serial só é aceito em runtime quando existe exatamente um dispositivo correspondente;
- seletores VID/PID que possam reivindicar a mesma unidade não podem coexistir silenciosamente entre providers;
- caminho HID explícito rejeita symlink e nó que não seja character device;
- caminho + seletor devem corresponder ao mesmo equipamento;
- HID configurado precisa estar presente para o provider declarar readiness;
- provider socket deve ser consumido com o tipo correto (`unix` ou `unixpacket`);
- packet↔packet preserva uma mensagem por escrita;
- transição `unixpacket`↔stream não pode perder fronteiras silenciosamente e exige framing explícito;
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
- Unix stream sockets;
- Unix `SOCK_SEQPACKET` (`unixpacket`);
- serial RS232/RS422/RS485 raw;
- USB HID Linux via `/dev/hidrawN`, com autodiscovery por VID/PID/serial, re-resolução por sessão e reports preservados em `unixpacket`;
- framing `length32be` opcional e explícito para transição packet↔stream;
- UDP preservando datagramas e sessões por peer;
- SocketCAN/CAN-FD preservando frames do ABI Linux;
- pair timeout, write timeout, half-close drain, keepalive, NODELAY e CIDR allowlist;
- métricas e sessões por transporte/direção.

### Limite específico de USB/ComAp

O provider HID é transporte, não conversor de protocolo. Para a InteliLite 4 AMF 9, ainda é obrigatório HIL para confirmar enumeração Linux, VID/PID, serial, interface, HID report descriptor, tamanhos/report IDs e o protocolo de aplicação sobre USB. Não declarar Modbus via USB nem compatibilidade com InteliConfig por emulação até existir evidência de bancada/documentação suficiente. O adapter ComAp Direct, se necessário, deve permanecer separado do core bridge-first.

O framing `length32be` também não transforma HID em Modbus; ele apenas preserva a fronteira dos reports quando um pacote precisa atravessar um stream TCP/Unix entre componentes compatíveis.

## Semântica de readiness

`/readyz` só fica verde quando:

- admin HTTP fez bind;
- cada stream/packet tunnel criou suas sources/listeners;
- cada UDP tunnel fez bind e resolveu target;
- cada serial provider publicou seu socket local;
- cada USB HID provider resolveu o equipamento configurado, validou o character device e publicou seu socket local;
- cada CAN provider encontrou a interface configurada e publicou seu socket local.

Readiness **não** significa que a controladora respondeu ao protocolo de aplicação. O HID está presente e validado, mas o handshake ComAp Direct continua sendo responsabilidade do consumidor/adapter e do HIL. Serial é aberto quando uma sessão chega; CAN raw é aberto quando um consumidor conecta.

Se um HID selecionado por VID/PID/serial for reenumerado com outro `hidrawN` depois de uma sessão cair, a sessão seguinte reexecuta a resolução estável. Isso melhora recuperação de hotplug, mas não transforma readiness em monitor dinâmico de presença do dispositivo.

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
17. validação de conteúdo do artifact, incluindo probe/documentação de campo;
18. artifact de release.

As actions de terceiros usam tags de versão principal no workflow atual, com `persist-credentials: false` no checkout. As ferramentas Go de supply chain permanecem pinadas. Pinagem imutável das actions por commit SHA é melhoria de supply chain pendente de revalidação do workflow.

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
- rollback automático e rollback manual com health gate;
- artifact leva `probe-usb-hid.sh`, README e documentação operacional relevante para diagnóstico em campo.

## Checkpoint standalone atual

- último checkpoint integralmente verde antes deste incremento: `1262dce7256b3fb6015ea1ccba126d460fe4be7f`;
- HEAD atual: **validation pending** até todos os jobs automatizados fecharem verdes novamente;
- depois dos gates automatizados, o status máximo continua sendo **software field-test-ready**;
- USB/ComAp só pode avançar para **production validated** após HIL físico.

## Gates físicos restantes para production validated

1. PUSR/USR real em reverse TCP → Gateway → Rapid/FUXA/consumidor;
2. dispositivo direto por IP/VPN;
3. RS232 real;
4. RS422 real quando aplicável;
5. RS485 real, incluindo direção/half-duplex do hardware;
6. USB HID real, incluindo InteliLite 4 AMF 9: enumeração, VID/PID/serial, descriptor, reports, read/write autorizado, permissões udev, unplug/replug e power-cycle;
7. adapter ComAp Direct em HIL se o caso USB não expuser protocolo diretamente consumível;
8. UDP real quando aplicável;
9. CAN clássico físico;
10. CAN-FD físico;
11. VPN/4G/MikroTik real;
12. power-cycle/reconnect de modem/controladora;
13. `tc netem`/impairment em HIL;
14. soak mínimo de 24 h, alvo de 7 dias;
15. rollback em máquina de homologação.

`scripts/run-soak.sh` aceita de 1 a 604800 segundos.

## Regra de promoção

- **validation pending**: código mudou e os novos gates ainda não fecharam;
- **software field-test-ready**: todos os gates automatizados/release/supply-chain do mesmo HEAD estão verdes;
- **production validated**: somente após HIL e soak físico da topologia real.

Não reintroduzir polling, mapas de memória, historian ou command plane genérico no core.
