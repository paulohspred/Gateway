# Matriz de fechamento software/LAB — 28 itens

Esta matriz acompanha a conclusão desta rodada. `PASS_CODE` significa implementação + testes locais; `PASS_TES` só pode ser marcado depois da release candidata ser instalada e validada na VM `tes`. HIL físico e produção são gates separados e não são inferidos desta matriz.

| # | Item | Código | tes | Evidência/critério |
|---:|---|---|---|---|
| 1 | Login + sessão protegida | PASS_CODE | PENDING_TES | RC Admin, cookie HttpOnly/SameSite, CSRF e BFF autenticado. |
| 2 | Troca obrigatória da senha bootstrap | PASS_CODE | PENDING_TES | Bootstrap não permanece no env de serviço; primeiro acesso exige troca. |
| 3 | MFA TOTP obrigatório | PASS_CODE | PENDING_TES | Administrator e Commissioning Engineer exigem MFA. |
| 4 | RBAC + siteScopes dos 6 roles | PASS_CODE | PENDING_TES | Viewer/Operator/Technician/Commissioning Engineer/Administrator/Auditor. |
| 5 | Administração de usuários | PASS_CODE | PENDING_TES | Criar/editar/resetar senha/MFA/revogar sessões; último Admin protegido. |
| 6 | Sites e metadados | PASS_CODE | PENDING_TES | ID integração, cliente/endereço/coords/contato opcionais e timezone validado. |
| 7 | Auditoria e sessões | PASS_CODE | PENDING_TES | Ações de segurança/admin auditadas; before/after sem segredos. |
| 8 | Visão Geral | PASS_CODE | PENDING_TES | KPIs factuais, críticos e comunicação; indisponibilidade não vira zero. |
| 9 | Frota Vertical | PASS_CODE | PENDING_TES | Cards operacionais com capabilities. |
| 10 | Frota Compacta | PASS_CODE | PENDING_TES | Cards compactos reais. |
| 11 | Frota Lista + capabilities | PASS_CODE | PENDING_TES | ABSENT/UNSUPPORTED separados de quality GOOD/STALE/OFFLINE/BAD/UNKNOWN. |
| 12 | Detalhe do Gerador | PASS_CODE | PENDING_TES | Identidade, controladora, motor, elétrica, combustível/DC, alarmes/eventos e quality. |
| 13 | Alarmes | PASS_CODE | PENDING_TES | Filtros e estados factuais; ACK industrial ausente. |
| 14 | Eventos | PASS_CODE | PENDING_TES | Timeline factual com período/tipo/busca. |
| 15 | Comunicação | PASS_CODE | PENDING_TES | Provider, comunicação por ativo e contagem de quality. |
| 16 | Histórico e Tendências | PASS_CODE | PENDING_TES | Archive real do Rapid; sem fallback inventado. |
| 17 | Diagnóstico e Saúde do Sistema | PASS_CODE | PENDING_TES | Controller→transport→Gateway→Rapid→Monitor→gerador e sessões autorizadas. |
| 18 | Configurações + Sobre/Versões | PASS_CODE | PENDING_TES | Preferências/padrões e Manifest/versões reais. |
| 19 | Commissioning Queue | PASS_CODE | PENDING_TES | Busca/filtros lifecycle/gates; DRAFT fora da Operação. |
| 20 | Wizard Adicionar Gerador | PASS_CODE | PENDING_TES | Ativo, controller/ECU, transporte, Rapid plan, monitorGeneratorId e profile sintético do LAB presente no catálogo de Engenharia. |
| 21 | Edição e imutabilidade de revisão | PASS_CODE | PENDING_TES | Edição apenas DRAFT; commissioned requer Change Commissioning. |
| 22 | Preflight automatizado | PASS_CODE | PENDING_TES | Estrutura, site, TCP reachability, profile lifecycle e RC Monitor/capabilities. |
| 23 | Validação automática de telemetria required | PASS_CODE | PENDING_TES | Online + required presentes + quality=good; gate não aceita PASS manual. |
| 24 | Profiles lifecycle | PASS_CODE | PENDING_TES | DRAFT/LAB/HIL_VALIDATED/HOMOLOGATED/DEPRECATED; HIL/HOMOLOGATED exigem evidência; simulator é agregado factual e permanece separado das famílias físicas DRAFT. |
| 25 | Rapid / Bindings seguro | PASS_CODE | PENDING_TES | cnlNum→MetricKey; sem registrador físico no browser. |
| 26 | Rapid plan staging/apply/rollback | PASS_CODE | PENDING_TES | Ferramenta privilegiada fora do browser; checksum, snapshot, rollback e Command API false. |
| 27 | Promoção/lifecycle/Change + pacote de evidências | PASS_CODE | PENDING_TES | 8 gates, profile canônico, promote/suspend/retire/supersede e export factual. |
| 28 | Release, instalação e recuperação LAB | PASS_CODE | PENDING_TES | SBOM Go+frontend, archive/installer contracts, deploy, outage/recovery/reboot e acceptance. |

## Guardrails

- `PRODUCTION_VALIDATED=false`.
- Nenhum profile é promovido automaticamente a `HIL_VALIDATED`/`HOMOLOGATED`.
- Nenhum comando START/STOP/RESET/TRANSFER/setpoint/ACK industrial é exposto.
- A nova VM limpa só deve ser criada depois de todos os 28 itens estarem `PASS_TES`.
