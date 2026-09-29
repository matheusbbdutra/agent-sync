# ADR — Compression Papers Reference (MEM1 + HiAgent)

**Status**: aceito
**Data**: 2026-09-29
**Decisor**: ctx-window maintainer
**Escopo**: fundamentação arquitetural — não implementa nada

## Contexto

Análise comparativa de 3 papers acadêmicos (MEM1, ACON, HiAgent) sobre compressão de contexto em agentes LLM long-horizon, feita como parte da exploração de projetos externos de token-optimizer (rtk, caveman, awesome-list).

Decidimos registrar neste ADR apenas os **2 papers que realmente fazem sentido arquitetural** para o agent-sync. O terceiro (ACON) entra como referência operacional para o `ctx-window benchmark` Phase 0+1 (framework de 3 eixos: tokens + task success + latency overhead), sem virar ADR próprio.

## Paper 1 — MEM1 (NeurIPS 2025)

**Referência**: Zhou et al., "MEM1: Learning to Synergize Memory and Reasoning for Efficient Long-Horizon Agents" (arXiv 2506.15841).

**Resumo em 1 parágrafo**: MEM1 reformula o trade-off memória vs raciocínio em agentes long-horizon via **RL end-to-end** que treina o LLM a manter memória constante ao longo da trajetória. A cada turno, o modelo funde nova observação + crenças anteriores em um **estado interno compacto compartilhado**. Reporta 3.5× performance vs Qwen2.5-14B full-context + 3.7× redução de memória em 16-objective multi-hop QA.

### Decisão: **NÃO implementar**

**Por que não**:
- MEM1 é **técnica de modelagem** (treinar/finetunar o LLM via RL), não arquitetura de middleware.
- Nosso caso é **middleware de sync cross-CLI** entre Claude/Codex/Cursor/Antigravity/Cline + OpenCode TS. Delegamos raciocínio a modelos externos em produção — não controlamos backbone.
- Custo proibitivo: fine-tunar 7B só para compressão de contexto num orquestrador não fecha economicamente.
- Janelas modernas (Claude Sonnet 4.5+, GPT-5, Gemini 2.5 = 1M+ tokens) reduziram urgência vs 2024.

**O que vale referenciar** (decisão registrada para futuro):
- A intuição de **"estado compartilhado compacto"** é útil conceitualmente — análoga ao nosso padrão de "uma memória canônica no Turso" em vez de cada CLI manter contexto divergente.
- **Compression-as-state-transition** ≈ nosso futuro "memory compaction job" rodando periodicamente sobre `memories` (já existe event-sourcing na arquitetura).
- Citar no STATE.md e em outros ADRs futuros para justificar a decisão de **single-source-of-truth**.

## Paper 2 — HiAgent (ACL 2025)

**Referência**: Hu et al., "HiAgent: Hierarchical Working Memory Management for Solving Long-Horizon Agent Tasks with Large Language Model" (arXiv 2408.09559).

**Resumo em 1 parágrafo**: HiAgent ataca working memory in-trial usando **subgoals como chunks**. Hierarquia de 2 níveis: (a) subgoal atual mantém todos os pares ação-observação detalhados; (b) subgoals passados mantêm apenas observação sumarizada. Quando o LLM decide que precisa de detalhe de subgoal passado, gera função de retrieval para re-expandir. Resultado: **+21pp success rate** (42 vs 21), **-35% contexto** vs standard prompting baseline, **-3.8 passos** médios.

### Decisão: **referência arquitetural, com evolução futura inspirada**

**O que vale implementar** (registrado para roadmap, **não agora**):

1. **Tag `subgoal_id` em memórias in-trial** — schema migration simples, sem mexer em scratch/permanent que já funciona.
2. **`ctx-window summarize-subgoal <id>`** — colapsa N memórias com mesmo subgoal em 1 summary + pointer (similar ao nosso `compact` mas em escopo menor).
3. **`ctx-window retrieve-subgoal <id> <query>`** — re-expande sob demanda quando LLM decide que precisa.

**Trade-offs explícitos**:
- Esforço: ~150 LOC para os 2 sub-comandos + schema change.
- Custo: 2 chamadas LLM adicionais por ciclo de subgoal (summarize + retrieve).
- Valor: -35% contexto no paper; precisa validação empírica no nosso workload.

**Por que não agora**:
- Schema change no memory-mcp requer cuidado com sync Turso cross-PC.
- Evidência do paper é em AgentBoard/text-games — não em coding agents (nosso caso).
- Risco de evicition policy prematura desperdiçar informação que parecia "summarizable" mas era crítica.

**Pré-requisito para implementar**: instrumentar `gain` (já wirado em heurística_compact + auto_compact) com entry `summarize` quando sub-comando for criado, para medir economia real antes de wirar automaticamente.

## O que NÃO está neste ADR (e por que)

- **ACON (ICLR 2026)** — referência operacional, não arquitetural. Seu framework de avaliação (3 eixos: tokens + task success + latency) já está parcialmente aplicado no `ctx-window benchmark` Phase 0 (tokens + latency). Task success é evolução planejada para Phase 1. Não merece ADR próprio — é refinamento do que já temos.
- **LLMLingua / LongLLMLingua / MemGPT / AOM** — tangenciais ao nosso caso (RAG/prompt compression clássica vs nosso estado persistente cross-CLI). Não citamos para não inflar o ADR.
- **rtk / caveman** — projetos de código, não papers. Análise já consolidada em [`docs/sprints/token-optimizer-aggregation.md`] (referência interna, a ser criada).

## Relação com outras decisões do repo

- **ADR-context-window-strategy.md** — define heurística fixa + sliding window K=5. Este ADR **não contradiz** — adiciona fundamentação acadêmica para roadmap futuro.
- **ADR-precompact-snapshot-cross-cli.md** — cross-PC summary sync (já mergeado em PR #5) é o **passo 0** da evolução HiAgent: já temos "memória canônica compartilhada". Próximo passo natural é adicionar subgoal-chunking por cima.

## Status e revisão

Este ADR é **aceito** (referência arquitetural). Não cria obrigação de implementação. Revisitar quando:
- Memória cross-PC saturar (>10k memories por projeto), OU
- Working memory de sessão real ultrapassar 50% do budget consistentemente (quality score D/F recorrente), OU
- ACON Phase 1 (task success scoring) revelar que compressão heurística degrada raciocínio real.

Em qualquer desses sinais, considerar abrir RFC para implementar features 1-3 do HiAgent acima.
