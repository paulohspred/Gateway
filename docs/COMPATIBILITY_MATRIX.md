# Matriz de compatibilidade — RC Universal Gateway

> Esta matriz descreve **transporte**, não suporte semântico a registradores/controladoras.

| Meio / caminho | Estado de software | Semântica do Gateway | Gate físico restante |
|---|---|---|---|
| TCP listen | field-test-ready quando CI final verde | raw stream | PUSR/modem real |
| TCP connect | field-test-ready quando CI final verde | raw stream | dispositivo IP/VPN real |
| TLS 1.3 | field-test-ready quando CI final verde | raw stream criptografado | certificados/topologia real |
| mTLS | field-test-ready quando CI final verde | raw stream + autenticação TLS | PKI/certificados reais |
| Unix stream | field-test-ready quando CI final verde | raw stream local | integração local alvo |
| Unix `SOCK_SEQPACKET` | implementado/testado | preserva uma mensagem por pacote | integração local alvo |
| `unixpacket` ↔ stream com `length32be` | implementado/testado | comprimento uint32 BE + payload original | cliente/adapter compatível |
| RS232 | field-test-ready quando CI final verde | bytes raw | porta/adaptador real |
| RS422 | field-test-ready quando CI final verde | bytes raw | porta/adaptador real |
| RS485 | field-test-ready quando CI final verde | bytes raw | half-duplex/direção do hardware |
| USB HID por `/dev/hidrawN` | transport implemented; CI cobre framing/segurança | preserva reports em `unixpacket`; write opt-in | dispositivo HID real + protocolo de aplicação |
| USB HID por VID/PID/serial | transport implemented; autodiscovery fail-closed | resolve/valida hidraw e preserva reports | VID/PID/serial reais + HIL |
| UDP | field-test-ready quando CI final verde | preserva datagramas e sessão por peer | dispositivo UDP real |
| SocketCAN clássico | field-test-ready em software | preserva ABI/frame em `unixpacket` | interface/transceiver físico |
| CAN-FD | field-test-ready em software | preserva ABI/frame FD em `unixpacket` | interface/transceiver FD físico |

## Consumidores validados por contrato

| Consumidor | Integração | Estado | Gate restante |
|---|---|---|---|
| Rapid SCADA v6 | `DrvCnlBasic` `TcpClient` → consumer TCP do Gateway | contrato/configuração preparado contra `scada-v6` master `1fd36080...` | executar Rapid SCADA real em VM |
| Rapid SCADA + Modbus TCP | `DrvModbus`, `TransMode=TCP` | perfis Gateway preparados | VM + dispositivo/servidor Modbus real |
| Rapid SCADA + RTU-over-TCP | `DrvModbus`, `TransMode=RTU` | perfis Gateway preparados | VM + modem/serial server real |
| Rapid SCADA + RS485 multidrop | `ConnectionMode=Shared`, `TransMode=RTU`, IDs no Rapid SCADA | perfil Gateway preparado | VM + barramento físico multidrop |

A integração Rapid SCADA não incorpora fonte/binário do Rapid SCADA ao Gateway. Consulte `RAPID_SCADA_INTEGRATION.md`.

## Protocolos que atravessam sem driver semântico

Quando o protocolo já é transportado pelo meio acima, o Gateway pode atuar como ponte sem conhecer seu significado.

| Família | Caminho típico | Observação |
|---|---|---|
| Modbus TCP | TCP/TLS | Rapid/FUXA/driver interpreta registradores |
| Modbus RTU/ASCII | Serial ou serial-over-TCP | Gateway não mantém mapa de memória |
| MQTT/MQTTS | TCP/TLS | Gateway não é broker nem converter |
| OPC UA | TCP/TLS | Gateway não modela nodes/tags |
| IEC 60870-5-104 | TCP/TLS/VPN | aplicação de destino interpreta |
| DNP3/TCP | TCP/TLS/VPN | aplicação de destino interpreta |
| HTTP/HTTPS | TCP/TLS | passthrough; Gateway não é proxy HTTP semântico |
| WebSocket/WSS | TCP/TLS | passthrough quando handshake/framing é entre os extremos |
| protocolo TCP proprietário | TCP/TLS | byte-transparent |
| IEC-101 / DNP3 serial / NMEA | Serial | byte-transparent |
| J1939 / CANopen | SocketCAN/CAN-FD | Gateway preserva frames; consumidor interpreta PGN/PDO |
| USB HID genérico | hidraw → `unixpacket` | Gateway preserva reports; consumidor interpreta protocolo HID/aplicação |
| ComAp Direct por USB | USB HID quando a controladora enumerar como hidraw | transporte implementado; adapter semântico não é prometido até HIL/documentação suficiente |

## Rapid SCADA: regra de `TransMode`

O Rapid SCADA deve receber o mesmo protocolo que o driver declara:

```text
native Modbus TCP      -> TransMode=TCP
Modbus RTU raw em TCP  -> TransMode=RTU
Modbus ASCII em TCP    -> TransMode=ASCII
```

O Gateway não converte RTU↔TCP. Para um único RS485 multidrop, use uma conexão compartilhada no Rapid SCADA; não crie múltiplos mestres concorrentes para o mesmo barramento.

## Regras importantes para USB HID

O nome `/dev/hidraw0`, `/dev/hidraw1` etc. pode mudar. Em produção, quando os metadados estiverem disponíveis, prefira selecionar a unidade por:

```text
vendorId + productId + serialNumber
```

Sem `serialNumber`, VID/PID só é aceito em runtime quando existe exatamente um match. Se `device` e seletor forem configurados simultaneamente, o Gateway confirma que ambos apontam para o mesmo hardware.

O provider só sinaliza readiness depois de resolver o equipamento, confirmar que o nó existe e que é character device e publicar o socket local.

## Framing de reports/frames sobre TCP

Não se deve assumir que um report HID ou frame CAN pode ser despejado em TCP sem perder sua fronteira. O Gateway rejeita uma configuração `unixpacket`↔stream sem framing explícito.

Com:

```json
"packetFraming": "length32be"
```

cada mensagem vira no stream:

```text
4 bytes de comprimento uint32 big-endian + payload original
```

Isso é um envelope do Gateway. Rapid SCADA, FUXA e drivers Modbus comuns **não entendem esse envelope automaticamente**; ele exige um cliente/adapter que conheça o contrato.

## InteliLite 4 AMF 9 por USB

O caminho USB está preparado no runtime para dispositivos Linux `hidraw`, inclusive com autodiscovery/validação de identidade. Para a InteliLite 4 AMF 9, a combinação precisa ser confirmada em bancada porque o conector USB A↔B não garante por si só qual interface/driver será exposto, quais reports serão usados ou que Modbus esteja disponível diretamente na USB.

O status correto antes do HIL é: **USB HID transport implemented / ComAp application adapter pending HIL**.

## O que não é uma promessa do core

- catálogo de todas as ComAp/DSE/PLCs/IHMs;
- banco de registradores;
- polling de registradores;
- historian;
- conversão para telemetria;
- broker MQTT;
- servidor OPC UA;
- interpretação J1939/CANopen;
- conversão automática ComAp Direct ↔ Modbus;
- Command Plane industrial.

## Rede/VPN

WireGuard, IPsec, OpenVPN, PPTP legado, APN privada, rotas MikroTik e outros túneis de rede pertencem ao plano de rede/SO. Para o Gateway, um endpoint roteável via VPN é simplesmente um endpoint TCP/UDP alcançável.

## Critério de status

- **implementado:** código existe;
- **software validated:** testes unitários/socket/race correspondentes passaram;
- **software field-test-ready:** todos os gates automatizáveis, release/supply-chain e documentação estão verdes no mesmo HEAD;
- **production validated:** somente depois de HIL + soak físico da combinação real de hardware/rede/consumidor.
