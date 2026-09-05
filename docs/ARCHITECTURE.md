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

### Frame

SocketCAN/CAN-FD usa `unixpacket` entre consumidor local e provider CAN para preservar a fronteira de cada frame do ABI Linux. `allowTransmit=false` mantém escrita em CAN bloqueada por padrão.

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

## Responsabilidades do core

1. validar configuração antes de abrir recursos;
2. abrir/aceitar transports;
3. aplicar allowlist/TLS quando configurado;
4. parear endpoints ou manter sessão por peer;
5. preservar bytes/datagramas/frames;
6. controlar reconnect, deadlines e shutdown;
7. manter sessões e métricas operacionais;
8. expor health/readiness em loopback.

## Fora do core

- mapas Modbus por fabricante/modelo;
- polling de registradores;
- interpretação de RPM/tensão/alarmes;
- historian/storage de processo;
- broker MQTT;
- servidor OPC UA;
- interpretação J1939/CANopen;
- command plane industrial genérico;
- fan-out raw sem arbitragem.

## Readiness

O runtime mantém `/readyz` falso até todos os componentes configurados inicializarem sua camada local:

- admin HTTP conseguiu bind;
- tunnels stream construíram suas sources/listeners;
- UDP conseguiu bind e resolver target;
- serial criou seu socket provider;
- CAN confirmou a interface configurada e criou seu socket provider.

Readiness não é HIL. Abertura efetiva de uma porta serial por sessão, comunicação com dispositivo remoto e tráfego CAN físico continuam sendo gates de campo.

## Concorrência e disponibilidade

- registry de sessões usa `RWMutex`;
- contadores de atividade usam atomics;
- métricas tiram snapshot sob lock e liberam o lock antes de qualquer escrita HTTP;
- erros fatais de componente cancelam o contexto interno e aguardam todas as goroutines antes de `Run` retornar;
- stream duplex aplica write deadline e drain de half-close;
- UDP revalida `lastSeen` antes de expirar uma sessão para evitar fechamento concorrente de sessão recém-ativada.

## Estrutura

```text
cmd/rc-gateway
      |
      v
internal/config
      |
      v
internal/gateway
   /    |      \
  v     v       v
bridge datagram providers
  |      |      /   \
 TCP    UDP  serial  CAN
 TLS
 Unix
      \
       admin + core/session + metrics
```

Packages principais:

- `internal/admin` — endpoints operacionais;
- `internal/bridge` — streams TCP/TLS/Unix;
- `internal/config` — schema e validações fail-closed;
- `internal/core` — sessões;
- `internal/datagram` — UDP;
- `internal/gateway` — lifecycle/orquestração;
- `internal/metrics` — registry Prometheus simples;
- `internal/provider/serialbridge` — serial raw;
- `internal/provider/canbridge` — SocketCAN/CAN-FD;
- `internal/transport/netutil` — CIDR allowlist.

## Extensões futuras

Novos meios entram somente como providers/transports que preservem a semântica do meio. WebSocket/WSS ou outro IPC podem ser adicionados se houver contrato duplex claro. Protocol-aware plugins devem permanecer separados do core bridge-first.
