# ADR — Teste manual direto de cartographer_preflight

- **Status**: Rejeitado — ver ADR-audit-removal-go.md (A-73) para o caminho Go adotado
- **Data**: 2026-09-25 (rev. 1: escopo mudou de wiramento para teste direto)
- **Decisor**: agente + usuário (ses_atual)
- **Fonte**: D-99 (investigação matrix escopos), D-100 (telemetria), cartographer (kingbootoshi) tem `preflight` + `audit_removal` + `audit_verify` + `diff` que repo-map não tem
- **Tags**: cartographer, preflight, manual-evaluation, no-wiring

## Contexto

`repo-map` (D-54, wirado em 5/5 CLIs) tem 3 tools:
- `get_file_impact` (blast radius, callers, tabelas DB, env vars)
- `get_symbol_callers` (rastreio reverso)
- `repo_summary` (hubs centrais)

[`kingbootoshi/cartographer`](https://github.com/kingbootoshi/cartographer) tem 10 tools, incluindo 4 que **não temos**:
- `cartographer_preflight` — valida impacto estrutural **antes** de Edit/Write
- `cartographer_audit_removal` — verifica se remoção quebrou referências
- `cartographer_audit_verify` — valida correção de edits
- `cartographer_diff` — diff estrutural (vs textual)

D-49/D-53 propuseram preflight originalmente mas foram descartadas em D-64 (subsumidas pelo repo-map minimalista).

## Problema real

Editamos código frequentemente. Não temos telemetria de **erros de edição** (variáveis renomeadas não-detectadas, símbolos quebrados, imports não-atualizados). `repo-map get_file_impact` ajuda **se o modelo chamar**, mas é opt-in.

## Decisão (rev. 1)

**Em vez de wirar cartographer como hook no agent-sync**, vamos **testar manualmente como ferramenta externa**. O agente (ou humano) chama `cartographer_preflight` **manualmente antes** de Edit em sessões de edição real, e decide se a saída agrega valor.

### Por que teste direto (não wiramento)

| Aspecto | Teste direto | Wiramento como hook |
|---|---|---|
| Setup | Instalar bun + cartographer (~10min) | Wiramento cross-CLI + sandbox (4-6h) |
| Custo reversão | Trivial (nada wirado) | Médio (rollback coordenado) |
| Decisão | Baseada em uso real | Baseada em smoke artificial |
| Overhead | Zero se não usar | ~100ms por Edit |
| Compatibilidade cross-CLI | N/A (uso manual) | Precisa wirar 5 CLIs |
| Telemetria | Manual (anotar uso) | Automática via repo-map |

**Pragmatismo (AGENTS.md §3)**: testar antes de comprometer. Se cartographer virar lixo, perdemos só o tempo do teste.

### Critérios de promoção (Proposto → Aceito)

| # | Critério | Como medir |
|---|---|---|
| 1 | `cartographer_preflight` detecta ≥1 erro em sessão real que modelo teria feito sem ele | anotação manual durante sessão |
| 2 | Latência percebida <1s para validação (não precisa ser <100ms — uso manual, não hook) | `time cartographer_preflight` |
| 3 | Saída legível e útil (não só blá-blá técnico) | revisão humana da saída |
| 4 | ROI pessoal >0 (você usaria de novo em próxima edição arriscada) | decisão subjetiva após 3-5 usos |

**Critérios NÃO exigidos** (anti-overengineering):
- Não precisa ser usado em 100% das edições
- Não precisa cobrir todos os tipos de erro
- Não precisa ser wirado (teste é opt-in por design)

### Quando viraria A-N de wiramento (se os critérios passarem)

Se `cartographer_preflight` for genuinamente útil:
- A-N nova: wirar como hook PreToolUse:Edit/Write em `apply_table.go`
- Esforço estimado: 4-6h (wiramento + smoke cross-CLI)
- Bloqueante: smoke deve ser aprovado (D-49/D-53 já rejeitaram wiramento antes)
- **Não automático** — decisão fica para depois do teste

## Consequências

**Positivas:**
- Validação baseada em uso real, não em smoke artificial
- Zero overhead no fluxo atual (opt-in)
- Sem wiramento cross-CLI = sem risco de regressão
- Reversibilidade trivial (nada foi modificado no agent-sync)

**Negativas:**
- Sem automação: precisa lembrar de chamar antes do Edit
- Sem telemetria de uso real
- Sem cobertura cross-CLI garantida (cada CLI precisa instalar Bun localmente)

**Trade-offs assumidos:**
- Teste direto é menos rigoroso que smoke wirado
- Resultado depende de uso real (subjetivo)
- Se cartographer virar padrão, wiramento vira A-N separada (não automática)

## Fora do escopo

- **A-73 cartographer_audit_removal**: mesma estrutura mas para delete/rename. Fica em espera até A-72 mostrar demanda de wiramento.
- **Migrar repo-map para Python**: não — perda de benefícios do binário Go single-file.
- **Wirar cartographer_preflight como hook**: rejeitado por escopo desta ADR. Só vira A-N se teste for positivo.
- **Substituir preflight por LLM-based check**: rejeitado (D-6 zero-LLM).

## Procedimento de teste

### Setup (10min)

```bash
# 1. Instalar Bun
curl -fsSL https://bun.sh/install | bash

# 2. Clonar cartographer
git clone https://github.com/kingbootoshi/cartographer.git /tmp/cartographer
cd /tmp/cartographer && bun install

# 3. Validar que MCP server inicia
bun run cartographer:mcp
# (deve conectar via stdio; Ctrl+C para sair)
```

### Uso manual em sessão de edição

Em uma sessão onde você pretende fazer Edit arriscado:

1. **Antes do Edit**: chamar `cartographer_preflight` manualmente
2. Fornecer contexto (arquivo, mudança pretendida)
3. Avaliar saída: detectou algo útil?
4. **Depois do Edit**: chamar `cartographer_audit_verify` ou `cartographer_audit_removal`
5. Validar que mudanças estão consistentes

### Avaliação após 3-5 usos

| Pergunta | Resposta |
|---|---|
| Detectou erros reais que teriam passado despercebidos? | sim/não |
| Latência foi aceitável para uso manual? | sim/não |
| Saída é legível e útil? | sim/não |
| Você usaria de novo em próxima edição arriscada? | sim/não |

**Se ≥3 sim**: merece virar A-N de wiramento (próxima decisão)
**Se <3 sim**: descarta A-72, mantém repo-map como está

### Documentação

Após testes, atualizar esta ADR com:
- Achados empíricos (latência medida, % erros detectados)
- Decisão: Aceitar (vira A-N wiramento) ou Rejeitar (mantém rejeitado)

## Implementação proposta (rev. 1)

Sequência de commits granulares:

1. `docs(adr) ADR-cartographer-preflight Proposto (rev. 1: teste direto)` — este arquivo atualizado
2. `docs(state) D-104 + A-72 [pending] rev. 1` — atualizar STATE.md
3. (FUTURO) `docs(investigation) cartographer-preflight-manual-test.md` — evidência dos testes manuais
4. (FUTURO, condicional) `docs(adr) ADR-cartographer-preflight Aceito (wiramento)` — só se teste for positivo

**Esforço teste**: ~1-2h (setup + 3-5 usos + documentação)
**Esforço wiramento** (futuro, condicional): 4-6h

## Reversibilidade

- ADR é documento — reversível por git revert
- Nenhum código foi modificado no agent-sync
- cartographer roda em `/tmp/` (sandbox isolado)
- Se teste for negativo: descartar ADR sem nenhum custo
- Se teste for positivo: wiramento vira A-N separada (escopo novo)

## Refs

- **D-49**: ADR-code-graph-bounded-context (descartado em D-64, fonte da inspiração)
- **D-53**: expansão repo-map (extratores relacionais)
- **D-54**: MCP server wirado em 5 CLIs
- **D-64**: auditoria ADRs (cartographer descartado)
- **D-99**: investigação matrix escopos (menciona cartographer)
- **D-100**: telemetria code-graph (A-70)
- **A-72**: smoke isolado de cartographer_preflight (escopo rev. 1)
- **kingbootoshi/cartographer**: https://github.com/kingbootoshi/cartographer (MIT, 10 tools)
- **Graphify-Labs/graphify**: https://github.com/Graphify-Labs/graphify (Apache-2.0, 7 tools, PR triage — não prioritário)