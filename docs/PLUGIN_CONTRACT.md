# Endpoint Provider Contract — direção bridge-first

> Leia primeiro [`PROJECT_STATE.md`](./PROJECT_STATE.md).

Plugins/providers existem para oferecer **endpoints duplex de comunicação** ao core bridge. Eles não convertem telemetria nem assumem semântica de registradores.

## Pergunta que um provider deve responder

> Como eu abro/aceito este meio e leio/escrevo bytes, datagramas, frames ou reports sem destruir a semântica do transporte?

Não:

> Qual registrador significa RPM, pressão ou alarme?

## Modelos de dados permitidos

Um provider deve declarar implicitamente pelo seu socket/API qual modelo entrega:

- **stream** — sequência de bytes sem fronteira de mensagem;
- **packet/frame/report** — uma leitura corresponde a uma unidade de mensagem que deve ser preservada;
- **datagram** — mensagem + identidade/sessão de peer quando aplicável.

No runtime atual, provider serial publica `unix` stream; USB HID e SocketCAN publicam `unixpacket`.

## Responsabilidades permitidas

- abrir/aceitar o transporte;
- Read/Write bidirecional;
- reconnect/lifecycle;
- descoberta e validação de identidade física quando necessária;
- TLS/mTLS quando aplicável;
- framing estritamente necessário para preservar a semântica do meio;
- limites explícitos de tamanho/recursos;
- status, métricas e erros operacionais;
- readiness local coerente com os recursos que o provider afirma disponibilizar.

## Regras de framing

Um provider não deve converter packet/frame/report em stream silenciosamente.

Se a fronteira de mensagem precisar atravessar um stream, o framing deve ser:

1. explícito na configuração/contrato;
2. reversível;
3. limitado em tamanho;
4. documentado;
5. testado nos dois sentidos;
6. transparente ao payload.

O contrato disponível hoje é `length32be`: `uint32 big-endian length + payload`.

## Identidade e fail-closed

Quando o SO fornece um nome de enumeração instável, o provider deve preferir uma identidade estável verificável. No HID, `/dev/hidrawN` pode ser resolvido/validado por VID/PID/serial.

Ambiguidade não pode escolher um dispositivo arbitrariamente. Zero/múltiplos matches são erro até o operador fornecer seletor suficiente.

## Segurança mínima de provider

- escrita/transmissão perigosa deve ser opt-in quando o meio permitir comportamento seguro read-only por padrão;
- symlinks e substituição de recursos locais devem ser avaliados antes de abrir device/socket;
- paths de IPC devem ser absolutos/canonicalizados;
- um provider não deve remover arquivo comum como se fosse socket stale;
- permissões do socket/device devem seguir least privilege;
- nenhum segredo ou credencial deve ser logado;
- erro fatal deve retornar ao orquestrador em vez de deixar goroutine órfã.

## Fora do contrato

- mapas de memória;
- polling semântico;
- converter valores de processo;
- historian/storage;
- alarmes de processo;
- broker MQTT;
- OPC UA model server;
- conversão automática entre protocolos industriais;
- command plane industrial genérico.

## Interface alvo conceitual

```text
Validate configuration
Resolve physical identity/resource
Open/Acquire
Read
Write
Close
Reconnect/lifecycle
Local/Remote identity metadata
Operational status / metrics
```

O provider deve poder ser pareado com outro endpoint pelo `Tunnel` sem exigir que o core conheça o protocolo de aplicação.

## Readiness

Um provider só deve sinalizar readiness depois que os recursos locais necessários para cumprir seu contrato estiverem válidos. Exemplos:

- serial: socket IPC publicado; porta física pode continuar lazy-open por sessão;
- HID: identidade resolvida, device node existente/validado e socket IPC publicado;
- CAN: interface SocketCAN existente e socket IPC publicado.

Readiness não equivale a resposta do equipamento/protocolo de aplicação.

## Dependências e licenças

Bibliotecas externas só devem ser introduzidas quando agregarem capacidade concreta que não possa ser obtida de forma mais simples/segura. Antes de incorporar código ou dependência, revisar licença, manutenção, supply chain e impacto no build reproduzível.

Referências externas podem orientar arquitetura, mas código incompatível com a política/licenciamento do projeto não deve ser copiado.

## Lifecycle de capacidade

```text
implemented -> software_validated -> field_test_ready -> production_validated
```

`production_validated` exige HIL e soak da combinação real. Command Plane permanece fora deste contrato.
