# Endpoint Provider Contract — direção bridge-first

> Leia primeiro [`PROJECT_STATE.md`](./PROJECT_STATE.md).

Plugins/providers futuros existem para oferecer **endpoints duplex de comunicação** ao core raw bridge. Eles não convertem telemetria.

## Pergunta que um provider deve responder

> Como eu abro/aceito este meio e leio/escrevo bytes ou datagramas com integridade?

Não:

> Qual registrador significa RPM, pressão ou alarme?

## Responsabilidades permitidas

- abrir/aceitar o transporte;
- Read/Write bidirecional;
- reconnect/lifecycle;
- TLS/mTLS quando aplicável;
- framing estritamente necessário ao meio;
- status e erros operacionais.

## Fora do contrato

- mapas de memória;
- polling semântico;
- converter valores de processo;
- historian/storage;
- alarmes;
- comandos industriais genéricos.

## Interface alvo conceitual

```text
Open/Acquire
Read
Write
Close
Reconnect/lifecycle
Local/Remote identity metadata
Operational status
```

O provider deve poder ser pareado com outro endpoint pelo `Tunnel`.

O antigo módulo de adapters de leitura foi removido. Bibliotecas externas só serão adicionadas novamente quando necessárias a um provider duplex concreto.

Lifecycle:

```text
experimental -> lab_validated -> field_validated -> production
```

Command Plane permanece fora deste contrato.
