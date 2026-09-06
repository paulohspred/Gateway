# RC Monitor — Operação e hardening

## Escopo

`rc-monitor` é o backend read-only da plataforma. Ele consome dados normalizados do Rapid SCADA, aplica o contrato semântico RC e expõe somente leitura em loopback.

Arquitetura:

```text
controladora -> RC Gateway -> Rapid SCADA -> Rapid Web API -> rc-monitor -> /api/v1
```

O Gateway continua sendo transporte. Endereços Modbus, function codes, byte/word order e polling permanecem no Rapid SCADA. O `rc-monitor` trabalha somente com números de canais Rapid e perfis canônicos.

## Segurança

- bind HTTP deve ser loopback;
- endpoint Rapid Web deve ser loopback;
- `AllowAuthApi=true` é necessário no Rapid Webstation;
- `AllowCommandApi` deve permanecer `false`;
- usuário/senha Rapid não são aceitos no JSON do monitor;
- credenciais são lidas de variáveis de ambiente indicadas por `usernameEnv` e `passwordEnv`;
- o unit systemd usa usuário dedicado `rc-monitor`, `NoNewPrivileges`, `PrivateDevices`, filesystem protegido e capability set vazio;
- o monitor não implementa START, STOP, transfer, reset ou setpoint.

## Configuração

Exemplos ficam em `configs/monitor/`.

Validação offline:

```bash
bin/rc-monitor --check-config --config /etc/rc-monitor.json
```

A validação carrega perfis e channel bindings, rejeita campos desconhecidos e não abre listener nem conexão Rapid.

Para `rapid-web`, um arquivo `/etc/rc-monitor.env` pode conter, por exemplo:

```text
RC_RAPID_USER=monitor
RC_RAPID_PASSWORD=trocar-por-segredo-real
```

Esse arquivo deve permanecer `0600 root:root` e não deve ser versionado.

## Instalação

A release RC contém `bin/rc-monitor`, `systemd/rc-monitor.service`, exemplos, controllers e `scripts/install-rc-monitor.sh`.

Depois de instalar a release Gateway/RC em `/opt/rc-gateway/current`:

```bash
sudo /opt/rc-gateway/current/scripts/install-rc-monitor.sh /caminho/rc-monitor.json /caminho/rc-monitor.env
```

O instalador:

1. valida a configuração com o binário da própria release;
2. cria `rc-monitor` como usuário de sistema sem shell;
3. instala `/etc/rc-monitor.json` com leitura restrita;
4. instala `/etc/rc-monitor.env` como `0600 root:root`;
5. instala e habilita o unit systemd;
6. exige `/readyz` saudável após restart.

Use `--dry-run` para validar pacote/config sem alterar o host.

## Endpoints

```text
GET /healthz
GET /readyz
GET /metrics
GET /api/v1/system/health
GET /api/v1/generators
GET /api/v1/generators/{id}
GET /api/v1/generators/{id}/telemetry
GET /api/v1/generators/{id}/alarms
GET /api/v1/generators/{id}/events
```

`/healthz` é liveness. `/readyz` depende do provider Rapid. `/metrics` expõe apenas métricas operacionais locais e não telemetria industrial.

## Semântica de qualidade

- valor Rapid com status `> 0`: definido;
- status `<= 0`: métrica ausente, nunca zero inventado;
- zero definido permanece zero;
- amostra antiga ultrapassando `staleAfterSeconds`: `stale`;
- falha de leitura: último valor conhecido pode ser retornado como `offline`;
- valores NaN/Inf, transform inválido ou enum não mapeado falham fechados.

## Alarmes e eventos

Alarmes e eventos são produzidos somente quando existe binding explícito. O monitor não deduz alarmes de nomes de fabricante nem de registradores Modbus.

O estado atual vem de canais Rapid. O histórico usa a Web API oficial do Rapid para eventos quando disponível. Se não houver evidência suficiente para determinar o instante de ativação, o sistema não fabrica timestamp de origem.

## Diagnóstico

```bash
systemctl status rc-monitor
journalctl -u rc-monitor --since today
curl -fsS http://127.0.0.1:18100/healthz
curl -fsS http://127.0.0.1:18100/readyz
curl -fsS http://127.0.0.1:18100/metrics
curl -fsS http://127.0.0.1:18100/api/v1/system/health
```

Nunca inclua `/etc/rc-monitor.env` em bundles de diagnóstico.

## Recovery

O serviço usa `Restart=on-failure`. Após falha/restart do Rapid, o monitor deve recuperar sem restart manual quando o endpoint volta a responder. MON-007 valida restart/recovery/mini-soak em software; a validação de VM/campo continua sendo gate separado.

## Limite de homologação

Profiles em estado `draft` não significam compatibilidade de produção. Suporte a uma controladora/modelo/firmware exige documentação permitida e/ou HIL, binding Rapid real e teste semântico. Escritas industriais permanecem bloqueadas.
