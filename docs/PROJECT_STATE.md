# RC Gateway / Generator Platform — Estado Canônico e Handoff

<!-- PROJECT_STATE_SCHEMA: 2 -->
<!-- CANONICAL_HANDOFF: true -->
<!-- CURRENT_CODE_BRANCH: feature/frontend-contract -->
<!-- CURRENT_DEVELOPMENT_TASK: REL-003 -->
<!-- EXTERNAL_RUNNING_GATE: none -->
<!-- PRODUCTION_VALIDATED: false -->
<!-- PR2_MUST_REMAIN_DRAFT: true -->

Este arquivo é o handoff canônico. Mudança material exige atualização posterior deste arquivo e `scripts/check-project-state-updated.sh` verde.

## Arquitetura fixa

```text
BRIDGE FIRST
PROTOCOL OPTIONAL
NO DEVICE MEMORY DATABASE
NO TELEMETRY HISTORIAN

campo -> RC GATEWAY (transporte)
      -> RAPID SCADA 6 (protocolo, registradores, polling, qualidade, histórico)
      -> RC MONITOR (domínio, API, segurança)
      -> FRONTEND
```

Regras: Gateway não contém register maps físicos; RC Monitor recebe canais Rapid, nunca endereços Modbus físicos; zero real permanece zero; dado indefinido permanece ausente; nenhuma escrita industrial nesta fase.

## Continuidade / branches

- `hardening/standalone-10x` -> PR #2 -> `main`: deve permanecer draft/not-merged.
- `feature/monitor-core` -> PR #3 -> `hardening/standalone-10x`: deve permanecer draft/not-merged.
- `feature/frontend-contract` -> PR #4 -> `feature/monitor-core`: deve permanecer draft durante UI-004.
- não alterar `main` nem mesclar PR #2/#3/#4 sem ordem explícita do proprietário.
- `tmp-backend-finish` permanece branch temporária de montagem/correção.

Rapid baseline: **6.4.7**. `PRODUCTION_VALIDATED=false`.

## SOAK-001 — relatório ainda não verificado

Janela registrada anteriormente:

```text
inicio: 2026-09-05T23:47:29Z
intervalo: 30 s
duração: 86400 s
fim esperado: 2026-09-06T23:47:29Z
```

O fim esperado já passou, mas o relatório real não foi verificado nesta sessão. Portanto `SOAK-001` não pode ser tratado como PASS nem como gate atualmente em execução. A VM original permanece intocada até localizar e validar o relatório real. Para DONE: `result=PASS`, todos os `bad_*`, restart/session-id changes e counter regressions iguais a zero; depois repetir production preflight non-root com `DISRUPTIVE=0`.

## Backend / segurança

Backend read-only permanece software-completo para o escopo atual. API:

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

Rapid Web usa Auth API habilitada e Command API obrigatoriamente desabilitada. `scadacomm6` executa non-root como `scadacomm`; `/opt/scada` permanece root-owned; somente o diretório dedicado de log é gravável pelo usuário do Communicator.

## UI-003 — DONE em deployment LAB reproduzível

Material validado antes deste handoff:

```text
material HEAD: 83340cb751efcce23bd74a0dfa0d8ed9aeee69b6
branch: feature/frontend-contract
Rapid SCADA: 6.4.7-1
Node: 22.23.2
npm: 10.9.8
Go: 1.27.1
release instalada na VM LAB: ui4-final-810aa84
```

Correções descobertas por instalação limpa e agora codificadas:

- caminho correto `/opt/scada/Config/ScadaInstanceConfig.xml` para Rapid SCADA 6.4.7;
- `rc-monitor` preservado como executável no release instalado;
- frontend incluído no artifact canônico e servido a partir de `/opt/rc-gateway/current/frontend`;
- `frontend/package-lock.json`, `npm ci` e Node exato pinado por `.node-version`/CI;
- Rapid Web API configurada por script com `AllowAuthApi=true` e `AllowCommandApi=false`;
- installer do RC Monitor exige paths absolutos para instalação em `/etc` e faz rollback para configuração known-good se o health gate falhar;
- installer do frontend/Nginx com backup, `nginx -t`, restart, health gate e rollback;
- acceptance de SPA, deep links, API read-only, 405, cache, headers e listener isolation;
- `install-rc-lab-stack.sh` orquestra stack-base -> Rapid Auth API -> Monitor -> frontend -> acceptance;
- o installer é fail-closed para Rapid SCADA sem checksum confiável; o ZIP oficial 6.4.7 usado no LAB está pinado em `48e8c8c33b8380fddc7c6012d6856123cb6328a1645e784e97a29aa525311180`, e fontes alternativas exigem `RC_SCADA_RAPID_SHA256` ou sidecar `.sha256`;
- release validator deixou de usar pipelines `tar | grep -q` sujeitos a SIGPIPE/141;
- Gateway CI teve comandos `run:` com quoting YAML inválido corrigidos para blocos; `actionlint` pinado passou no workflow corrigido;
- `scripts/ci.sh` agora valida configs de Monitor com path absoluto de runtime no contexto equivalente do checkout, sem confundir `/etc`/`/opt` com source tree;
- binding demo é explicitamente `SIMULATION_TEST_ONLY`; alarme é `SIMULATED_DIGITAL_ALARM`; o stock demo não arquiva eventos, portanto o binding LAB não inventa evento.

Evidência na segunda VM `tes`:

```text
release archive validation: PASS
application installer contract: PASS
go test ./...: PASS
frontend npm ci: PASS
frontend typecheck: PASS
frontend tests: 5/5 PASS
frontend production build: PASS
full LAB orchestrator: PASS
rc-gateway: active
rc-monitor rapid-web: active/healthy
scadaagent6/scadaserver6/scadacomm6/scadaweb6: active
nginx: active
rc-scada-internal-firewall: active
Rapid Web outage -> Monitor offline/503: PASS
Rapid Web recovery -> Monitor healthy/good: PASS
RC Monitor restart recovery: PASS
RC Monitor failed-provider rollback: PASS
SPA top-level + deep-link refresh: PASS
POST read-only endpoint -> 405: PASS
static cache + HTTP security headers: PASS
10000/10002 loopback allowed and LAN blocked: PASS
zero defined -> value=0 quality=good: PASS
undefined channel -> metric absent, not zero: PASS
SIMULATED_DIGITAL_ALARM raise: PASS
SIMULATED_DIGITAL_ALARM clear: PASS
/events on stock Rapid demo: [] by design; no fake history inserted
Gateway CI workflow actionlint after YAML fix: PASS
Local CI script parity with required tooling: PASS
```

Final LAB state records:

```text
mode=SIMULATION_TEST_ONLY
rapid_provider=rapid-web
rapid_commands=false
physical_controller=false
production_validated=false
```

Isso fecha serving/deployment reproduzível do frontend e a vertical Rapid -> RC Monitor -> frontend em ambiente simulado. Não fecha SEM real, HIL ou produção.

## Frontend / UI-004 — DONE; software/LAB 28/28 validado na `tes`

Rotas do produto atual:

```text
/
/generators
/generators/:id
/alarms
/events
/history
/communication
/settings
/about
/account/security
/engineering/commissioning
/engineering/commissioning/new
/engineering/commissioning/:id
/engineering/commissioning/:id/edit
/engineering/profiles
/engineering/profiles/:profileId
/engineering/bindings
/engineering/diagnostics
/admin/users
/admin/roles
/admin/sites
/admin/audit
/admin/system
```

Capabilities e histórico estão conectados ao backend real. A HMI distingue `ABSENT` de `UNSUPPORTED` e preserva `good/stale/offline/bad/unknown` e zero real. Login/MFA/RBAC/CSRF protegem o BFF operacional; o Nginx não publica o RC Monitor operacional diretamente. Sem START/STOP/RESET/TEST/TRANSFER/setpoints/acknowledge.

A matriz de fechamento dos 28 itens software/LAB está em `docs/SOFTWARE_LAB_28.md` e está em **28/28 `PASS_TES`**. O fechamento combina prova live da instalação (primeiro acesso/MFA, Monitor/Rapid, outage/recovery/restart/reboot) e testes de integração do commit exato executados na própria `tes` para RBAC/Sites/Commissioning; não implica objetos de teste persistentes nem HIL físico.

O catálogo de Engenharia agrega também o pacote factual `rc-simulator.reference-controller` diretamente de `controllers/rc-simulator/reference-controller/{manifest,telemetry,alarms}.json`; ele não é inserido nas nove famílias físicas de `DRAFT_PROFILES.json`. Isso permite o commissioning LAB do `gen-sim-001` com o mesmo `profileId` informado por `/capabilities`, sem sugerir HIL/homologação.


## REL-003 — fechamento software/LAB 2026-09-07

Estado validado na `tes`:

```text
software/LAB: 28/28 PASS_TES
source/GitHub HEAD: e1ca27e5026382be2a8195279589f23593c84336
source/GitHub tree: 0663359cf27a356fd329195590dd43dbad997fba
release instalada: ui4-final-810aa84
release Manifest commit: 810aa846a9177e410fe0acb3ed05f878ca258bfb
admin bootstrap password change: PASS live
admin MFA TOTP: PASS live
outage/recovery: PASS live
restart: PASS live
cold boot: PASS live
post-boot required telemetry: GOOD
post-boot Rapid history: HTTP 200, 2 series
release reproducibility local AMD64+ARM64: PASS byte-a-byte
Rapid SCADA 6.4.7 Linux ZIP SHA256 pin: 48e8c8c33b8380fddc7c6012d6856123cb6328a1645e784e97a29aa525311180
PRODUCTION_VALIDATED=false
```

A release atualmente em execução (`ui4-final-810aa84`) contém o runtime já validado na `tes` e referencia `810aa846` no Manifest. O HEAD `e1ca27e` adiciona apenas hardening de release/installer/documentação: SBOM frontend determinístico para builds byte-a-byte e pinagem fail-closed do pacote Rapid SCADA. REL-003 só fecha depois de CI/CodeQL remoto verde neste HEAD, proteção/merge de `main`, rebuild da release a partir do SHA final de `main` e smoke final na `tes`.

## Política de controladoras

GenMon é referência funcional/factual clean-room, não fonte para copiar código/JSON. `controllers/DRAFT_PROFILES.json` contém vocabulário RC; endereços/FC/encoding reais permanecem no Rapid SCADA e só são promovidos com documentação permitida e/ou HIL. Primeira homologação de controladora real continua read-only.

## Checklist canônico

<!-- CHECKLIST_START -->
| ID | Status | Critério |
|---|---|---|
| GW-001 | DONE | Gateway bridge-first. |
| GW-002 | DONE | Rapid transport comprovado. |
| GW-003 | DONE | Firewall Rapid validado. |
| VM-001 | DONE | Stack instalada/validada. |
| VM-002 | DONE | Rapid -> Gateway -> simulador comprovado. |
| VM-003 | DONE | Field outage/recovery. |
| VM-004 | DONE | Blackhole/recovery. |
| VM-005 | DONE | Cold boot. |
| VM-006 | DONE | ScadaComm non-root. |
| VM-007 | DONE | Cold boot pós-non-root. |
| HARD-001 | DONE | Installer ScadaComm non-root least-privilege. |
| SOAK-001 | TODO | Janela terminou; localizar e verificar relatório real antes de tocar a VM original. |
| SEM-001 | TODO | Binding semântico com canais Rapid reais, não demo. |
| MON-001 | DONE | Foundation. |
| MON-002 | DONE | API read-only. |
| MON-003 | DONE | Profiles. |
| MON-004 | DONE | RapidScadaProvider. |
| MON-005 | DONE | Rapid Web + semântica + E2E software. |
| MON-006 | DONE | Hardening/release/observabilidade. |
| MON-007 | DONE | Restart/recovery/mini-soak software. |
| HIL-001 | BLOCKED | Primeira controladora real read-only; depende de hardware. |
| HIL-002 | BLOCKED | Modem/VPN/meio físico; depende de hardware/meio. |
| CMD-001 | DEFERRED | Writes somente após HIL/interlocks/autorização/auditoria. |
| UI-001 | DONE | Contratos de produto/HMI/commissioning congelados. |
| UI-002 | DONE | Shell + primeira vertical frontend real. |
| UI-003 | DONE | Release/frontend/Nginx/orquestrador LAB reproduzível e E2E Rapid -> Monitor -> frontend validado em VM limpa. |
| UI-004 | DONE | Capabilities/history, edge cases de quality, responsividade/a11y e superfícies operacionais/admin/engenharia implementadas; validação final pertence a REL-003. |
| REL-001 | TODO | Confirmar proteção de main. |
| REL-002 | DONE | Release inclui Gateway, Monitor e frontend; SBOM frontend foi normalizado e AMD64/ARM64 são reproduzíveis byte-a-byte em teste local. |
| REL-003 | IN_PROGRESS | 28/28 software/LAB PASS_TES; HEAD `e1ca27e` alinhado GitHub/local. Faltam CI remoto verde, proteção/merge de main e release final construída do SHA de main com smoke na `tes`. |
| VM-CLEAN-001 | NEXT | Criar VM Ubuntu limpa somente depois de REL-003 DONE e 28/28 PASS_TES. |
| PROD-001 | BLOCKED | Exige SOAK verificado + SEM real + HIL + aprovação. |
<!-- CHECKLIST_END -->

## Próximo passo exato

```text
1. publicar este handoff atualizado e obter CI + CodeQL remoto verde no HEAD final;
2. configurar proteção de `main` conforme `docs/GITHUB_PROTECTION.md`;
3. promover o PR #5 para `main` somente com todos os gates verdes;
4. congelar o SHA final de `main` e marcar o handoff de release;
5. construir release AMD64+ARM64 com SBOM a partir do SHA final de `main`;
6. reinstalar essa release na `tes`, preservar `RC_FRONTEND_BIND=0.0.0.0:80` e repetir acceptance/Manifest/Git;
7. marcar REL-003 DONE;
8. somente então iniciar VM-CLEAN-001; HIL/SEM/SOAK de campo permanecem gates posteriores antes de `PRODUCTION_VALIDATED=true`.
```

`PRODUCTION_VALIDATED=false`. HIL físico, SEM real de campo e produção permanecem gates externos separados.
