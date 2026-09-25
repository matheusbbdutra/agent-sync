# ADR — Smoke isolado de cartographer_preflight

- **Status**: Proposto
- **Data**: 2026-09-25
- **Decisor**: agente + usuário (ses_atual)
- **Fonte**: D-99 (investigação matrix escopos), D-100 (telemetria), cartographer (kingbootoshi) tem `preflight` + `audit_removal` + `audit_verify` + `diff` que repo-map não tem
- **Tags**: cartographer, preflight, cross-cli, smoke-validation

## Contexto

`repo-map` (D-54, wirado em 5/5 CLIs) tem 3 tools:
- `get_file_impact` (blast radius, callers, tabelas DB, env vars)
- `get_symbol_callers` (rastreio reverso)
- `repo_summary` (hubs centrais)

[`kingbootoshi/cartographer`](https://github.com/kingbootoshi/cartographer) tem 10 tools, incluindo 3 que **não temos**:
- `cartographer_preflight` — hook pré-edição que valida impacto estrutural antes de Edit/Write
- `cartographer_audit_removal` — verifica se remoção quebrou referências
- `cartographer_audit_verify` — valida correção de edits
- `cartographer_diff` — diff estrutural (vs textual)

D-49/D-53 propuseram preflight originalmente mas foram descartadas em D-64 (subsumidas pelo repo-map minimalista).

## Problema real

Editamos código frequentemente. Não temos telemetria de **erros de edição** (variáveis renomeadas não-detectadas, símbolos quebrados, imports não-atualizados). `repo-map get_file_impact` ajuda se o modelo chamar, mas é opt-in.

## Decisão

Smoke isolado de **1 sessão real** usando `cartographer_preflight` como hook PreToolUse:Edit/Write. Medir se:

1. Reduz erros de edição empíricos (variáveis quebradas, imports não-atualizados)
2. Tempo de execução do hook (deve ser <100ms para não degradar UX)
3. Compatibilidade cross-CLI (5 CLIs suportadas?)
4. Custo de dependência (cartographer é Python + tree-sitter vs nosso repo-map Go nativo)

### Critérios de promoção (Proposto → Aceito)

| # | Critério | Como medir |
|---|---|---|
| 1 | Smoke 1 sessão: preflight detecta ≥1 erro que modelo teria feito sem hook | sessão isolada com edit arriscado |
| 2 | Tempo de execução do hook <100ms em 95% dos casos | instrumentar `--telemetry` ou `time` |
| 3 | Wiramento cross-CLI funciona em ≥3 de 5 CLIs | smoke wiramento |
| 4 | Decisão sobre dependência: Python wrapper OK ou exigir re-implementar em Go | análise de tradeoff |

**Critérios NÃO exigidos** (anti-overengineering):
- Não precisa cobrir 100% dos tipos de erro
- Não precisa substituir `get_file_impact` (são complementares)
- Não precisa replicar tree-sitter — se for lento, descartar

## Consequências

**Positivas (se Aceito):**
- Redução empírica de erros de edição
- Feedback estrutural antes do Edit (vs reativo depois)
- Complementa `get_file_impact` (post-edit) com pre-edit

**Negativas:**
- Mais uma dependência no runtime (Python + tree-sitter)
- Latência adicionada por Edit (~100ms estimado)
- Complexidade de wiramento cross-CLI

**Trade-offs assumidos:**
- Smoke isolado primeiro (esforço baixo, decisão baseada em dado)
- Se falhar, descartar sem comprometer a arquitetura repo-map
- Não substituir `repo-map` — apenas somar

## Fora do escopo

- **A-73 cartographer_audit_removal**: mesma estrutura mas para delete/rename. Fica em espera até A-72 mostrar demanda.
- **Migrar repo-map para Python**: não — perda de benefícios do binário Go single-file.
- **Substituir preflight por LLM-based check**: rejeitado (D-6 zero-LLM).

## Implementação proposta

Sequência de commits granulares (1 commit por peça):

1. `docs(adr) ADR-cartographer-preflight Proposto` — este arquivo
2. `docs(state) D-104 + A-72 [pending]` — catalogar
3. (FUTURO) `chore(smoke) install cartographer em sandbox` — wiramento de teste
4. (FUTURO) `docs(smoke) SMOKE-TEST-V.md` — procedimento do smoke
5. (FUTURO) `docs(smoke) cartographer-preflight-sess-1.md` — evidência

**Esforço smoke**: 4-6h (instalar + wirar 1 CLI + rodar sessão + medir).

## Reversibilidade

- ADR é documento — reversível por git revert
- Smoke isolado em sandbox — não toca wiramento de produção até Aceito
- Se falhar critérios 1-4, descartar ADR (mantém como Proposto indefinido ou marca Rejeitado)

## Refs

- **D-49**: ADR-code-graph-bounded-context (descartado em D-64, fonte da inspiração)
- **D-53**: expansão repo-map (extratores relacionais)
- **D-54**: MCP server wirado em 5 CLIs
- **D-64**: auditoria ADRs (cartographer descartado)
- **D-99**: investigação matrix escopos (menciona cartographer)
- **D-100**: telemetria code-graph (A-70)
- **kingbootoshi/cartographer**: https://github.com/kingbootoshi/cartographer (MIT, 10 tools)
- **Graphify-Labs/graphify**: https://github.com/Graphify-Labs/graphify (Apache-2.0, 7 tools, PR triage — não prioritário)