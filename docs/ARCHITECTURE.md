# RC Universal Gateway — Arquitetura

> Estado/handoff canônico: [`PROJECT_STATE.md`](./PROJECT_STATE.md).

## Missão

O RC Universal Gateway é uma ponte universal de conectividade industrial. Ele transporta dados entre o campo e um consumidor sem incorporar semântica de dispositivo.

```text
BRIDGE FIRST
PROTOCOL OPTIONAL
NO DEVICE MEMORY DATABASE
NO TELEMETRY HISTORIAN
```

## Modelos de transporte

### Stream

```text
FIELD ENDPOINT <======== raw duplex bytes ========> CONSUMER ENDPOINT
```

Implementações atuais:

- TCP `listen`/`connect`;
- TLS 1.3/mTLS sobre TCP;
- Unix stream sockets;
- RS232/RS422/RS485 expostos por provider Unix local.

Em `listen ↔ connect`, a conexão inbound é o trigger para discar o lado `connect`. Em `listen ↔ listen`, o runtime pareia uma conexão de cada lado.

### Datagram

UDP possui runtime próprio para preservar fronteiras de datagrama. Cada peer inbound recebe sessão isolada para o endpoint target, limitada por `maxSessions`, `maxDatagramBytes` e `idleTimeoutSeconds`.

### Packet / frame / report

`unixpacket` (`AF_UNIX/SOCK_SEQPACKET`) é o IPC local para meios nos quais a fronteira da mensagem faz parte do contrato. O bridge genérico suporta esse network diretamente.

- USB HID publica um report por mensagem `unixpacket`;
- SocketCAN/CAN-FD publica um frame por mensagem `unixpacket`;
- packet↔packet usa um único `Read` + um único `Write` por mensagem, evitando divisão silenciosa de pacotes;
- `allowWrite=false` em HID e `allowTransmit=false` em CAN mantêm escrita/transmissão bloqueadas por padrão.

### Transição packet ↔ stream

TCP e Unix stream não preservam fronteiras de mensagem. Portanto, um túnel que misture `unixpacket` com um stream é inválido sem framing explícito.

O framing implementado é:

```text
packetFraming = length32be

4 bytes uint32 big-endian: tamanho do payload
N bytes: payload original
```

O limite atual é 64 KiB por pacote. Esse envelope preserva o transporte; não transforma HID/CAN em Modbus nem em qualquer protocolo de aplicação.

## Identidade USB HID

`/dev/hidrawN` é um nome de enumeração, não uma identidade persistente. O provider HID aceita:

- caminho explícito `/dev/hidrawN`; ou
- `vendorId` + `productId` + `serialNumber` opcional.

Sem caminho explícito, o runtime consulta sysfs e exige exatamente um dispositivo correspondente. Zero ou múltiplos matches falham. Quando caminho e seletor são fornecidos juntos, a identidade é verificada antes de readiness.

## Fluxos típicos

### PUSR reverso + Rapid

```text
Controladora -> PUSR -> rede/VPN -> Gateway field listen :15003
                                      || raw stream
                           consumer listen :25003 <- Rapid
```

### Equipamento direto por VPN/IP

```text
Gateway field connect -> 10.60.20.222:502
         || raw stream
consumer listen :25020 <- Rapid/FUXA
```

### USB HID + adapter local

```text
Controladora USB
      |
 /dev/hidrawN  <- identidade validada por VID/PID/serial quando configurado
      |
 USB HID provider
      |
 unixpacket (1 report = 1 mensagem)
      |
 adapter de aplicação / consumidor compatível
```

Se o adapter estiver remoto e a fronteira dos reports precisar atravessar TCP:

```text
USB HID -> unixpacket -> length32be/TCP -> cliente compatível -> adapter
```

## Responsabilidades do core

1. validar configuração antes de abrir recursos;
2. abrir/aceitar transports;
3. aplicar allowlist/TLS quando configurado;
4. parear endpoints ou manter sessão por peer;
5. preservar bytes, datagramas, frames e reports conforme o modelo do meio;
6. exigir framing explícito quando uma fronteira packet precisa atravessar stream;
7. controlar reconnect, deadlines e shutdown;
8. manter sessões e métricas operacionais;
9. expor health/readiness em loopback.

## Fora do core

- mapas Modbus por fabricante/modelo;
- polling de registradores;
- interpretação de RPM/tensão/alarmes;
- historian/storage de processo;
- broker MQTT;
- servidor OPC UA;
- interpretação J1939/CANopen;
- conversão ComAp Direct ↔ Modbus;
- command plane industrial genérico;
- fan-out raw sem arbitragem.

## Readiness

O runtime mantém `/readyz` falso até todos os componentes configurados inicializarem sua camada local:

- admin HTTP conseguiu bind;
- tunnels stream/packet construíram suas sources/listeners;
- UDP conseguiu bind e resolver target;
- serial criou seu socket provider;
- USB HID resolveu o equipamento configurado, validou o character device e criou seu socket provider;
- CAN confirmou a interface configurada e criou seu socket provider.

Readiness não é HIL. A existência do HID não comprova o handshake ComAp Direct; comunicação com dispositivo remoto, serial real e tráfego CAN físico continuam sendo gates de campo.

## Concorrência e disponibilidade

- registry de sessões usa `RWMutex`;
- contadores de atividade usam atomics;
- métricas tiram snapshot sob lock e liberam o lock antes de qualquer escrita HTTP;
- erros fatais de componente cancelam o contexto interno e aguardam todas as goroutines antes de `Run` retornar;
- stream duplex aplica write deadline e drain de half-close;
- packet duplex preserva uma escrita atômica por mensagem;
- UDP revalida `lastSeen` antes de expirar uma sessão para evitar fechamento concorrente de sessão recém-ativada.

## Configuração e recursos

`LoadStrict` lê o arquivo uma vez, rejeita campos JSON desconhecidos/trailing e aplica normalização/validação sobre a mesma fotografia de bytes. A validação de produção também conhece o tipo dos provider sockets:

```text
serial     -> unix
USB HID    -> unixpacket
SocketCAN  -> unixpacket
```

Um túnel que tente consumir um provider com network incompatível é rejeitado antes do startup.

## Estrutura

```text
cmd/rc-gateway
      |
      v
internal/config
      |
      v
internal/gateway
   /      |        \
  v       v         v
bridge  datagram  providers
  |       |       /   |   \
 TCP     UDP  serial HID  CAN
 TLS          unix   \   /
 Unix               unixpacket
 unixpacket
      \
       admin + core/session + metrics
```

Packages principais:

- `internal/admin` — endpoints operacionais;
- `internal/bridge` — TCP/TLS/Unix/unixpacket e framing packet↔stream;
- `internal/config` — schema e validações fail-closed;
- `internal/core` — sessões;
- `internal/datagram` — UDP;
- `internal/gateway` — lifecycle/orquestração;
- `internal/metrics` — registry Prometheus simples;
- `internal/provider/serialbridge` — serial raw;
- `internal/provider/usbhid` — HID raw com identidade estável opcional;
- `internal/provider/canbridge` — SocketCAN/CAN-FD;
- `internal/transport/netutil` — CIDR allowlist.

## Extensões futuras

Novos meios entram somente como providers/transports que preservem a semântica do meio. WebSocket/WSS ou outro IPC podem ser adicionados se houver contrato duplex claro. Protocol-aware adapters devem permanecer separados do core bridge-first.
