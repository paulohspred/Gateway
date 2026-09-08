# RC Monitor Frontend — mapa de superfícies de referência

Status: inventário de referência para UI-001. Este documento **não autoriza implementação** de funcionalidades industriais e não altera o backend.

Fonte auditada: `paulohspdev-cmyk/ProjetoGerador`, branch `main`, com comparação do menu atual e do menu original/importado em `9e29b0d195bc605f3ff6574c5dc5ec49036efa44` e pré-simplificação em `d5e52ce06d9bbe6e14071a56ea300f99315f33d2`.

## Regra de contrato

O frontend novo não deve ser construído a partir de lembrança, mockup ou promessa verbal. Cada superfície futura precisa nascer deste inventário e receber uma decisão explícita: `KEEP`, `REDESIGN`, `DEFER`, `DROP` ou `REFERENCE_ONLY`.

Imagens conceituais são referência visual, não prova de endpoint, dado, permissão ou capacidade. Nenhuma ação de escrita industrial é habilitada sem contrato de backend e gate específico.

## Resumo auditado

- 54 conceitos de tela/menu aparecem no conjunto histórico do frontend antigo.
- 37 itens permanecem no menu atual do repositório antigo.
- desses 37, 29 são menu operacional comum e 8 pertencem ao grupo administrativo condicionado a permissão.
- 16 superfícies continuam roteáveis, mas foram removidas da navegação principal e ficam como rotas técnicas ocultas/admin.
- `Sites` é uma superfície órfã: existia no menu original, continua registrada no roteador, mas hoje não está nem no menu principal nem na lista oficial de itens técnicos ocultos.
- além disso existem as rotas independentes de login e detalhe individual do gerador.

## Menu atual visível — operador

| Grupo | Item | Slug/rota | Estado no legado |
|---|---|---|---|
| Operação | Visão Geral | `/` | visível |
| Operação | Geradores | `/p/geradores` | visível |
| Operação | Central de Operação | `/p/central-de-operacao` | visível |
| Operação | Alarmes | `/p/alarmes` | visível |
| Operação | Eventos | `/p/eventos` | visível |
| Operação | Mapa | `/p/mapa` | visível |
| Comunicação | Modems | `/p/modems` | visível |
| Comunicação | Conectividade | `/p/conectividade` | visível |
| Comunicação | Gateways | `/p/gateways` | visível |
| Comunicação | Comunicação | `/p/comunicacao` | visível |
| Energia | Rede | `/p/energia-rede` | visível |
| Energia | Geradores | `/p/energia-geradores` | visível |
| Energia | Carga | `/p/energia-carga` | visível |
| Energia | Transferência | `/p/energia-transferencia` | visível |
| Energia | Paralelismo | `/p/energia-paralelismo` | visível |
| Manutenção | Manutenção | `/p/manutencao` | visível |
| Manutenção | Combustível | `/p/combustivel` | visível |
| Manutenção | Baterias | `/p/baterias` | visível |
| Manutenção | Horímetros | `/p/horimetros` | visível |
| Manutenção | Agenda | `/p/agenda` | visível |
| Manutenção | Histórico | `/p/historico` | visível |
| Manutenção | Relatórios | `/p/relatorios` | visível |
| Gestão | Clientes | `/p/clientes` | visível |
| Gestão | Unidades | `/p/unidades` | visível |
| Automação | Regras | `/p/regras` | visível |
| Automação | Exercício automático | `/p/exercicio-automatico` | visível |
| Automação | Agendamentos | `/p/agendamentos` | visível |
| Automação | Notificações | `/p/notificacoes` | visível |
| Automação | Escalonamento | `/p/escalonamento` | visível |

## Menu atual condicionado a administrador

O `AppSidebar` remove este grupo para quem não possui a permissão usada pelo legado para administração.

| Grupo | Item | Slug/rota | Estado no legado |
|---|---|---|---|
| Administração | Usuários | `/p/usuarios` | admin-only |
| Administração | Perfis e permissões | `/p/perfis` | admin-only |
| Administração | Controladoras | `/p/controladoras` | admin-only |
| Administração | Configurações | `/p/configuracoes` | admin-only |
| Administração | Saúde do sistema | `/p/saude` | admin-only |
| Administração | Backups | `/p/backups` | admin-only |
| Administração | Auditoria | `/p/auditoria` | admin-only |
| Administração | Versão | `/p/versao` | admin-only |

## Rotas técnicas ocultas no menu atual

Estas superfícies continuam reconhecidas pelo roteador legado, mas não aparecem na navegação diária. O `findItem` as trata como grupo administrativo.

| Grupo técnico | Item atual | Slug/rota | Nome histórico quando diferente |
|---|---|---|---|
| Monitoramento | Tendências | `/p/tendencias` | Tendências |
| Sistema | Canais | `/p/canais` | Canais |
| Sistema | Tags | `/p/tags` | Tags |
| Sistema | Templates | `/p/templates` | Templates |
| Sistema | Motor de telemetria | `/p/rapid-scada` | Rapid SCADA |
| Sistema | Diagnóstico | `/p/diagnostico` | Diagnóstico |
| Biblioteca | Fabricantes | `/p/fabricantes` | Fabricantes |
| Biblioteca | Controladoras | `/p/lib-controladoras` | Controladoras |
| Biblioteca | Protocolos | `/p/protocolos` | Protocolos |
| Biblioteca | Perfis homologados | `/p/controller-packs` | Controller Packs |
| Biblioteca | Laboratório | `/p/laboratorio` | Laboratório |
| Integrações | API | `/p/api` | API |
| Integrações | Webhooks | `/p/webhooks` | Webhooks |
| Integrações | E-mail | `/p/email` | E-mail |
| Integrações | WhatsApp | `/p/whatsapp` | WhatsApp |
| Integrações | ERP / BMS / outros | `/p/erp-bms` | ERP / BMS / outros |

## Superfície órfã detectada

| Item | Slug/rota | Evidência | Risco |
|---|---|---|---|
| Sites | `/p/sites` | `SitesScreen` permanece em `registry.ts`; era item visível no menu original | o roteador consegue abrir a tela, mas `findItem` não a classifica; breadcrumb/título/permissão ficam inconsistentes |

`Sites` deve receber decisão explícita no frontend novo; não deve ser ressuscitado automaticamente só porque existe no legado.

## Rotas independentes de menu

| Superfície | Rota | Observação |
|---|---|---|
| Login | `/login` | existe no legado; só entra no novo produto se houver contrato real de autenticação |
| Detalhe do gerador | `/p/geradores/:id` | superfície principal de equipamento; forte candidata a `REDESIGN` |

## Estrutura histórica antes da simplificação

No frontend original, todos os 54 conceitos estavam expostos no menu em 11 grupos:

| Grupo histórico | Itens |
|---|---|
| RC Geradores | Visão Geral; Geradores; Central de Operação; Sites; Mapa |
| Monitoramento | Alarmes; Eventos; Tendências; Histórico; Relatórios |
| Energia | Rede; Geradores; Carga; Transferência; Paralelismo |
| Manutenção | Manutenção; Combustível; Baterias; Horímetros; Agenda |
| Equipamentos | Controladoras; Modems; Gateways; Conectividade; Comunicação |
| Automação | Regras; Exercício automático; Agendamentos; Notificações; Escalonamento |
| SCADA | Canais; Tags; Templates; Rapid SCADA; Diagnóstico |
| Biblioteca | Fabricantes; Controladoras; Protocolos; Controller Packs; Laboratório |
| Gestão | Clientes; Unidades; Usuários; Perfis e permissões; Auditoria |
| Integrações | API; Webhooks; E-mail; WhatsApp; ERP / BMS / outros |
| Sistema | Configurações; Backups; Saúde do sistema; Versão |

A simplificação posterior não apagou a maior parte das telas: ela reorganizou grupos, moveu itens administrativos e escondeu superfícies técnicas.

## Diferenças relevantes entre o histórico e o menu atual

| Histórico | Situação atual |
|---|---|
| `Sites` em RC Geradores | órfão: rota registrada, sem menu e sem classificação técnica |
| Alarmes / Eventos em Monitoramento | movidos para Operação |
| Tendências em Monitoramento | oculto/admin |
| Histórico / Relatórios em Monitoramento | movidos para Manutenção |
| Equipamentos | grupo removido; comunicação virou grupo próprio; Controladoras foi para Administração |
| SCADA | grupo removido do menu; todas as superfícies ficaram ocultas/admin |
| Biblioteca | grupo removido do menu; todas as superfícies ficaram ocultas/admin |
| Gestão | Clientes/Unidades permanecem; Usuários/Perfis/Auditoria foram para Administração |
| Integrações | grupo removido do menu; todas as superfícies ficaram ocultas/admin |
| Sistema | grupo removido; seus itens foram para Administração |

## Itens vistos apenas nas imagens de referência

As imagens fornecidas mostram conceitos como `Telemetria`, `Plano preventivo`, menus e ações de operação/manutenção que não aparecem com esses nomes no `nav.ts` original nem no atual auditados acima. Portanto, neste momento, eles são `REFERENCE_ONLY`: servem para linguagem visual e UX, não entram automaticamente no escopo funcional.

Da mesma forma, botões ou ações mostrados em imagens (`START`, `STOP`, `Desligar`, `Testar`, reconhecer alarme, transferência, nova OS etc.) não criam capacidade no produto novo.

## Gate antes de implementar UI

Antes de escrever as telas do novo frontend, criar uma matriz de decisão com uma linha para cada uma das 54 superfícies e preencher:

- decisão: `KEEP`, `REDESIGN`, `DEFER`, `DROP` ou `REFERENCE_ONLY`;
- endpoint atual do RC Monitor que sustenta a tela;
- dados realmente disponíveis;
- estado de qualidade aplicável (`good`, `stale`, `offline`, `bad`, `unknown`, ausente);
- comportamento read-only;
- referência visual aprovada;
- critério de teste.

Nenhuma tela fora dessa matriz deve aparecer por iniciativa da implementação.