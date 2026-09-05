# Matriz de compatibilidade — RC Universal Gateway

> Esta matriz descreve **transporte**, não suporte semântico a registradores/controladoras.

| Meio / caminho | Estado de software | Semântica do Gateway | Gate físico restante |
|---|---|---|---|
| TCP listen | field-test-ready quando CI final verde | raw stream | PUSR/modem real |
| TCP connect | field-test-ready quando CI final verde | raw stream | dispositivo IP/VPN real |
| TLS 1.3 | field-test-ready quando CI final verde | raw stream criptografado | certificados/topologia real |
| mTLS | field-test-ready quando CI final verde | raw stream + autenticação TLS | PKI/certificados reais |
| Unix socket | field-test-ready quando CI final verde | raw stream local | integração local alvo |
| RS232 | field-test-ready quando CI final verde | bytes raw | porta/adaptador real |
| RS422 | field-test-ready quando CI final verde | bytes raw | porta/adaptador real |
| RS485 | field-test-ready quando CI final verde | bytes raw | half-duplex/direção do hardware |
| UDP | field-test-ready quando CI final verde | preserva datagramas e sessão por peer | dispositivo UDP real |
| SocketCAN clássico | field-test-ready em software | preserva ABI/frame | interface/transceiver físico |
| CAN-FD | field-test-ready em software | preserva ABI/frame FD | interface/transceiver FD físico |

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

## O que não é uma promessa do core

- catálogo de todas as ComAp/DSE/PLCs/IHMs;
- banco de registradores;
- polling de registradores;
- historian;
- conversão para telemetria;
- broker MQTT;
- servidor OPC UA;
- interpretação J1939/CANopen;
- Command Plane industrial.

## Rede/VPN

WireGuard, IPsec, OpenVPN, PPTP legado, APN privada, rotas MikroTik e outros túneis de rede pertencem ao plano de rede/SO. Para o Gateway, um endpoint roteável via VPN é simplesmente um endpoint TCP/UDP alcançável.

## Critério de status

- **implementado:** código existe;
- **software validated:** testes unitários/socket/race correspondentes passaram;
- **software field-test-ready:** todos os gates automatizáveis, release/supply-chain e documentação estão verdes no mesmo HEAD;
- **production validated:** somente depois de HIL + soak físico da combinação real de hardware/rede/consumidor.
