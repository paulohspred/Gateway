# RC Universal Gateway — Architecture

> Handoff canônico: [`PROJECT_STATE.md`](./PROJECT_STATE.md).

## Missão

O RC Universal Gateway é uma **ponte universal de conectividade**. Ele conecta dois endpoints e transporta bytes nos dois sentidos sem precisar entender a semântica do equipamento.

```text
BRIDGE FIRST
PROTOCOL OPTIONAL
NO DEVICE MEMORY DATABASE
NO TELEMETRY HISTORIAN
```

## Unidade fundamental: Tunnel

```text
FIELD ENDPOINT  <======== raw duplex bytes ========>  CONSUMER ENDPOINT
```

Os dois lados são simétricos e podem ser `listen` ou `connect`. No milestone atual, o core suporta TCP.

### PUSR TCP Client + Rapid

```text
Controladora -> PUSR -> Internet -> MikroTik -> Gateway field listen :15003
                                                     ||
                                                raw tunnel
                                                     ||
                                       consumer listen :25003 <- Rapid
```

### Equipamento direto por VPN/IP

```text
Gateway field connect -> 10.60.20.222:502
         ||
     raw tunnel
         ||
consumer listen :25020 <- Rapid
```

Em `listen ↔ connect`, o peer inbound é o trigger: só depois dele existir o Gateway disca o outro lado.

## Responsabilidades do core

1. abrir/aceitar transporte;
2. parear endpoints;
3. aplicar segurança de conexão/allowlist;
4. rotear túnel;
5. encaminhar bytes sem modificação;
6. reconnect/lifecycle;
7. health, readiness, sessões, logs e métricas operacionais.

## Fora do core

- banco de registradores;
- mapas Modbus por fabricante/modelo;
- polling de pontos;
- conversão de RPM/pressão/tensão/alarme;
- historian/storage de processo;
- banco obrigatório de dispositivos;
- engine de alarmes/dashboard;
- fan-out semântico de telemetria.

Um protocolo desconhecido deve atravessar o túnel raw normalmente.

## Framing/protocolo opcional

Framing só entra quando necessário para **transportar corretamente**, por exemplo conversão serial/encapsulamento ou arbitragem especializada. Conhecer framing nunca autoriza o core a conhecer o significado de registradores.

## Sem fan-out raw

Uma sessão request/response não pode ser copiada cegamente para múltiplos mestres. Um Tunnel raw possui um consumidor ativo por vez. Compartilhamento de dados ocorre depois do driver/SCADA/broker ou por plugin protocol-aware separado.

## Observabilidade

Métricas do Gateway descrevem a ponte, não o processo industrial: pares, conexões, bytes por direção, erros, reconnects e readiness.

## Estrutura atual

```text
cmd/rc-gateway
      |
      v
internal/config
      |
      v
internal/gateway
      |
      v
internal/bridge
   /       \
field    consumer
```

Packages mantidos no caminho principal: `admin`, `bridge`, `config`, `core/session`, `gateway`, `metrics` e `transport/netutil`.

O antigo módulo `adapters/`, event bus, parsers/detectores, ingest, spool e leitores semânticos foram removidos após a decisão bridge-first.

## Extensões futuras corretas

A expansão ocorre por novos **endpoint providers duplex**:

- TLS/mTLS;
- Serial RS232/422/485;
- UDP com política explícita de sessão;
- WebSocket/WSS;
- Unix socket/local IPC;
- SocketCAN/CAN-FD quando houver contrato duplex apropriado;
- MQTT apenas quando houver contrato de mensagens request/response adequado;
- roteamento dinâmico seguro para muitos modems.

Cada novo provider precisa provar preservação de payload e comportamento de reconnect antes de integrar o runtime.
