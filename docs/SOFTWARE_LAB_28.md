# Matriz de fechamento software/LAB — 28 itens

Esta matriz acompanha a conclusão desta rodada. `PASS_CODE` significa implementação + testes locais; `PASS_TES` significa que o critério foi exercitado na VM `tes`, por prova live da instalação ou por teste de integração do commit exato executado na própria VM, conforme explicitado abaixo. HIL físico e produção são gates separados e não são inferidos desta matriz.

| # | Item | Código | tes | Evidência/critério |
|---:|---|---|---|---|
| 1 | Login + sessão protegida | PASS_CODE | PASS_TES | RC Admin, cookie HttpOnly/SameSite, CSRF e BFF autenticado. |
| 2 | Troca obrigatória da senha bootstrap | PASS_CODE | PASS_TES | Bootstrap não permanece no env de serviço; primeiro acesso exige troca. |
| 3 | MFA TOTP obrigatório | PASS_CODE | PASS_TES | Administrator e Commissioning Engineer exigem MFA. |
| 4 | RBAC + siteScopes dos 6 roles | PASS_CODE | PASS_TES | Viewer/Operator/Technician/Commissioning Engineer/Administrator/Auditor. |
| 5 | Administração de usuários | PASS_CODE | PASS_TES | Criar/editar/resetar senha/MFA/revogar sessões; último Admin protegido. |
| 6 | Sites e metadados | PASS_CODE | PASS_TES | ID integração, cliente/endereço/coords/contato opcionais e timezone validado. |
| 7 | Auditoria e sessões | PASS_CODE | PASS_TES | Ações de segurança/admin auditadas; before/after sem segredos. |
| 8 | Visão Geral | PASS_CODE | PASS_TES | KPIs factuais, críticos e comunicação; indisponibilidade não vira zero. |
| 9 | Frota Vertical | PASS_CODE | PASS_TES | Cards operacionais com capabilities. |
| 10 | Frota Compacta | PASS_CODE | PASS_TES | Cards compactos reais. |
| 11 | Frota Lista + capabilities | PASS_CODE | PASS_TES | ABSENT/UNSUPPORTED separados de quality GOOD/STALE/OFFLINE/BAD/UNKNOWN. |
| 12 | Detalhe do Gerador | PASS_CODE | PASS_TES | Identidade, controladora, motor, elétrica, combustível/DC, alarmes/eventos e quality. |
| 13 | Alarmes | PASS_CODE | PASS_TES | Filtros e estados factuais; ACK industrial ausente. |
| 14 | Eventos | PASS_CODE | PASS_TES | Timeline factual com período/tipo/busca. |
| 15 | Comunicação | PASS_CODE | PASS_TES | Provider, comunicação por ativo e contagem de quality. |
| 16 | Histórico e Tendências | PASS_CODE | PASS_TES | Archive real do Rapid; sem fallback inventado. |
| 17 | Diagnóstico e Saúde do Sistema | PASS_CODE | PASS_TES | Controller→transport→Gateway→Rapid→Monitor→gerador e sessões autorizadas. |
| 18 | Configurações + Sobre/Versões | PASS_CODE | PASS_TES | Preferências/padrões e Manifest/versões reais. |
| 19 | Commissioning Queue | PASS_CODE | PASS_TES | Busca/filtros lifecycle/gates; DRAFT fora da Operação. |
| 20 | Wizard Adicionar Gerador | PASS_CODE | PASS_TES | Ativo, controller/ECU, transporte, Rapid plan, monitorGeneratorId e profile sintético do LAB presente no catálogo de Engenharia. |
| 21 | Edição e imutabilidade de revisão | PASS_CODE | PASS_TES | Edição apenas DRAFT; commissioned requer Change Commissioning. |
| 22 | Preflight automatizado | PASS_CODE | PASS_TES | Estrutura, site, TCP reachability, profile lifecycle e RC Monitor/capabilities. |
| 23 | Validação automática de telemetria required | PASS_CODE | PASS_TES | Online + required presentes + quality=good; gate não aceita PASS manual. |
| 24 | Profiles lifecycle | PASS_CODE | PASS_TES | DRAFT/LAB/HIL_VALIDATED/HOMOLOGATED/DEPRECATED; HIL/HOMOLOGATED exigem evidência; simulator é agregado factual e permanece separado das famílias físicas DRAFT. |
| 25 | Rapid / Bindings seguro | PASS_CODE | PASS_TES | cnlNum→MetricKey; sem registrador físico no browser. |
| 26 | Rapid plan staging/apply/rollback | PASS_CODE | PASS_TES | Ferramenta privilegiada fora do browser; checksum, snapshot, rollback e Command API false. |
| 27 | Promoção/lifecycle/Change + pacote de evidências | PASS_CODE | PASS_TES | 8 gates, profile canônico, promote/suspend/retire/supersede e export factual. |
| 28 | Release, instalação e recuperação LAB | PASS_CODE | PASS_TES | SBOM Go+frontend, archive/installer contracts, deploy, outage/recovery/reboot e acceptance. |

## Evidência de fechamento na `tes` — 2026-09-07

- Código fonte e GitHub `feature/frontend-contract`: tree Git `23935277fc754e4c42fa2b2dd5e9dd7704ce823b`; HEAD remoto/local `d2e741a4d8e19cdd3da88857ceaefe23d611342c`.
- Primeiro acesso real: senha bootstrap alterada; `mustChangePassword=false`; MFA TOTP concluído pelo operador e persistido com `mfaEnabled=true`.
- RBAC, administração de usuários, Sites e Commissioning: suíte `internal/control` do HEAD exato executada na `tes`, incluindo lifecycle, seis roles, siteScopes, último Administrator, sessões, Change Commissioning, profile lifecycle e gates; PASS. O estado persistido de LAB foi mantido limpo (um Administrator; sem Site/Commissioning permanente), portanto esta evidência não afirma objetos de teste persistentes.
- Frontend: typecheck PASS; Vitest 19/19 PASS; `npm audit --audit-level=high` = 0 vulnerabilidades; BFF/Nginx acceptance PASS.
- CI local do HEAD exato: `scripts/ci.sh` PASS; cobertura global 62,3% (mínimo 58%); `internal/control` 64,7%; race detector PASS; builds/configs/Rapid Plan contract PASS.
- Monitor/Rapid live: `gen-sim-001` online; required `engine.rpm`, `generator.voltage_l1`, `generator.frequency` com `quality=good`; histórico real HTTP 200 com duas séries; sem fallback sintético de histórico.
- Outage controlado de `scadaweb6`: Monitor `503/degraded`, provider unavailable, `communication=offline`, zero métricas `GOOD`; últimos valores preservados com quality `offline`. Recovery: required retornaram `GOOD`.
- Restart controlado de `rc-gateway`, `rc-monitor`, `rc-admin` e `nginx`: PASS; acceptances pós-restart PASS.
- Cold boot final: boot ID mudou de `b73eb8b2-fc3e-46e0-8183-dc87abe7b0b1` para `5d16875d-2bb0-4251-85ef-84e96a95d410`; oito serviços habilitados/ativos; Admin/Monitor/frontend HTTP 200; acceptances pós-boot PASS; telemetria required `GOOD`; histórico real HTTP 200.
- Rapid plan privilegiado: dry-run/snapshot/apply no-op/rollback e `AllowCommandApi=false` já provados na `tes`; comandos industriais permanecem ausentes.

**Resultado software/LAB: 28/28 `PASS_TES`.** Isso não é declaração de HIL físico nem de produção.

## Guardrails

- `PRODUCTION_VALIDATED=false`.
- Nenhum profile é promovido automaticamente a `HIL_VALIDATED`/`HOMOLOGATED`.
- Nenhum comando START/STOP/RESET/TRANSFER/setpoint/ACK industrial é exposto.
- A nova VM limpa só deve ser criada depois de todos os 28 itens estarem `PASS_TES`.
