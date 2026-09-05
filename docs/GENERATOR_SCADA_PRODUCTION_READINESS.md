# SCADA de geradores — readiness Gateway + Rapid SCADA v6

## Escopo desta fase

Esta fase valida somente a camada de aquisição/comunicação formada por:

```text
Controladora / gerador / modem / VPN / RS485 / Ethernet
                        |
                  RC Universal Gateway
                        |
                 TCP local previsível
                        |
              Rapid SCADA v6 Communicator
```

Backend de negócio, API própria, frontend, dashboards adicionais e integrações posteriores ficam fora deste gate.

## Versão Rapid SCADA de referência

Baseline revisado: Rapid SCADA **v6.4.7**, repositório `RapidScada/scada-v6`, branch `master`, commit `1fd36080c7830303f921672fdaee335a06e7ae50`.

A integração usa `DrvCnlBasic`/`TcpClient` e `DrvModbus` nativos. `ConnectionMode=Shared` significa uma conexão compartilhada para todos os devices de uma communication line e é o padrão recomendado quando um tunnel representa um único barramento RS485.

## Regra para chamar de produção

Não existe um selo genérico de “100%” para todas as controladoras. O status de produção é concedido por matriz efetivamente testada:

```text
modelo da controladora
+ firmware
+ protocolo e mapa de registradores
+ meio físico / modem / VPN
+ parâmetros seriais/rede
+ versão do Gateway
+ versão do Rapid SCADA
+ configuração/template do Rapid SCADA
```

A automação de repositório pode provar o contrato de software. VM, equipamento físico e comportamento do firmware precisam de evidência de bancada/HIL.

## Gates automatizados do Gateway

O candidato precisa passar no mesmo HEAD:

- `gofmt`, `go vet`, Staticcheck, module integrity e workflow lint;
- unit/integration tests com shuffle;
- race detector;
- cobertura mínima;
- validação estrita de todos os exemplos de configuração;
- concorrência real, 1.000 pares e 1.000 churns TCP;
- impairment e mini-soak;
- `govulncheck` e CodeQL;
- segurança do instalador;
- release reproduzível amd64/arm64;
- SHA256, SBOM, dry-run e conteúdo obrigatório do artifact;
- contrato explícito Rapid SCADA/Modbus.

## Contrato Rapid SCADA/Modbus automatizado

`internal/bridge/rapid_scada_test.go` cobre explicitamente:

1. Modbus TCP function 03 — leitura de holding registers;
2. Modbus TCP function 06 — escrita de single register;
3. Modbus TCP function 16 — escrita de multiple registers;
4. exception response Modbus TCP;
5. Modbus RTU raw sobre TCP com CRC preservado;
6. requests e responses de escrita RTU com CRC preservado;
7. cinco Unit IDs no mesmo stream persistente, representando RS485 multidrop compartilhado;
8. fragmentação e coalescência de TCP sem corrupção de stream;
9. 1.000 ciclos consecutivos de polling Modbus TCP em uma única sessão.

Esses testes não transformam o Gateway em parser Modbus. Eles apenas provam que o data plane preserva exatamente o tráfego que o Rapid SCADA espera enviar e receber.

## Gate com Rapid SCADA real na VM

Depois de instalar o artifact exato do Gateway e Rapid SCADA v6 real:

```bash
sudo RAPID_SCADA_COMM_SERVICE=scadacomm6.service \
  RAPID_SCADA_EXPECT_SESSION=1 \
  /opt/rc-gateway/current/scripts/rapid-scada-acceptance.sh
```

O teste exige Gateway ativo/ready, Communicator ativo e ao menos uma sessão real Rapid SCADA -> Gateway.

Para o preflight de produção, informe explicitamente as portas consumer do Gateway:

```bash
sudo RAPID_SCADA_GATEWAY_PORTS="25020" \
  RAPID_SCADA_MAX_RESTARTS=0 \
  RAPID_SCADA_REQUIRE_NTP=1 \
  /opt/rc-gateway/current/scripts/rapid-scada-production-acceptance.sh
```

O preflight verifica também boot enablement, listeners somente em loopback, sincronismo de relógio, espaço livre, restart counters e integração real.

Em VM descartável ou janela de manutenção autorizada, acrescente:

```bash
RAPID_SCADA_DISRUPTIVE=1
```

para provar recuperação após restart do Communicator e do Gateway.

## Hardening Rapid SCADA que precisa ser decidido em VM

O unit `scadacomm6.service` do Rapid SCADA v6.4.7 upstream usa `Type=notify`, `Restart=always` e não declara `User=`. Em um system service, a ausência de `User=` significa execução como root.

O preflight informa esse fato. Para exigir non-root:

```bash
RAPID_SCADA_REQUIRE_NON_ROOT=1
```

Não aplicar um override arbitrário sem teste: primeiro é necessário ajustar ownership/permissões de `/opt/scada`, logs e demais caminhos realmente gravados pelo produto e executar novamente todos os gates. A promoção final deve registrar se o ambiente foi homologado non-root ou qual controle compensatório foi aceito.

## Critério de rede para co-localização

Quando Rapid SCADA e Gateway estiverem na mesma VM, os endpoints consumer do Gateway devem permanecer em loopback:

```text
127.0.0.1:<porta>
```

Rapid SCADA não precisa ser exposto diretamente ao modem/controladora. O Gateway concentra reconnect, VPN/LAN endpoint, serial provider e limites do data plane.

## RS485 multidrop de geradores

Para um barramento físico único:

- uma communication line Rapid SCADA;
- `ConnectionMode=Shared`;
- um mestre efetivo;
- Unit IDs únicos;
- mesmos baud/data bits/parity/stop bits;
- terminação e bias corretos;
- polling dimensionado para não saturar o barramento.

Não usar várias conexões TCP concorrentes contra o mesmo RS485 esperando que o Gateway faça arbitragem semântica.

## Segurança de comandos de geradores

O Gateway é duplex. Portanto, se o Rapid SCADA emitir um comando Modbus válido, o Gateway o transportará.

Para a primeira homologação de cada modelo/firmware:

- começar com projeto Rapid SCADA somente leitura;
- validar todos os registradores de medição e escalas;
- validar qualidade/stale/offline;
- somente depois habilitar comandos em bancada controlada;
- documentar function code, register, valor, interlock e efeito físico esperado;
- testar falha de rede durante e após comando;
- manter autenticação/autorização/auditoria no SCADA.

Start, stop, transfer, reset de alarmes, mudança de setpoint e outros comandos que alterem o gerador não entram no gate inicial de leitura.

## HIL mínimo por modelo de controladora

Para declarar um modelo `production_validated`, registrar:

- fabricante/modelo/firmware;
- protocolo real usado;
- mapa/template e versão;
- 100% dos pontos essenciais comparados com display/software oficial;
- leitura contínua sem corrupção;
- desconectar/reconectar cabo/rede/modem;
- power-cycle da controladora;
- restart do Gateway;
- restart do Rapid SCADA Communicator;
- perda e retorno da VPN/4G;
- comportamento de stale/offline;
- latência de polling observada;
- CPU/RAM/FDs durante carga;
- 24 h de soak mínimo; alvo de 7 dias antes de rollout amplo.

Quando comandos forem habilitados, cada comando precisa de HIL separado e evidência do estado final do equipamento.

## Critério de promoção

```text
CI + CodeQL verdes no mesmo HEAD
        -> software_field_test_ready
Rapid SCADA real na VM
        -> vm_accepted para essa integração
controladora/modem/VPN reais
        -> hil_accepted para essa matriz
24 h mínimo / 7 dias alvo
        -> soak_accepted
aprovação operacional da matriz
        -> production_validated
```

Enquanto qualquer etapa externa estiver pendente, o termo correto é `software_field_test_ready`, não “100% produção”.
