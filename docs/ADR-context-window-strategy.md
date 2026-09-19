# ADR: Context Window Strategy (Sliding Window + Summary)

**Status**: Aceito
**Data**: 2026-09-17
**Decisor**: Matheus Dutra
**Tags**: ai-agent, context-management, performance, hooks

## Contexto

Sessões longas de LLM degradam em silêncio: o agente ignora regras, contradiz decisões anteriores, e "esquece" o que está no **meio** da janela (*lost in the middle*). Compaction automática do provedor descarta instruções sem avisar. Sem uma estratégia explícita, o agente-sync dependia apenas da disciplina do modelo (skill `context-guard` + nudge hooks), o que é frágil.

Hipótese a validar: aplicar o padrão do paper **"Less Context, Better Agents: Efficient Context Engineering for Long-Horizon Tool-Using LLM Agents"** (Lodha, Pahlavikhah Varnosfaderani, Chakraborty, Mithal — Microsoft, arXiv [2606.10209v1](https://arxiv.org/html/2606.10209v1)).

## Decisão

Implementar a estratégia **sliding window + incremental summary**, alinhada ao paper:

1. **Working memory (alta fidelidade)**: últimos K tool calls verbatim.
   - K padrão: **5** (alinhado ao paper, §4.4).
   - K configurável por sessão via `ctx-window set-k`.

2. **Compressed memory (sumário)**: gerada por LLM sob demanda quando o working memory estoura (tokens > budget ou N tool calls > threshold).
   - Formato YAML estruturado com 6 seções: `decisions`, `active_hypotheses`, `artifacts`, `resolved_errors`, `next_steps`, `constraints` (ver `skills/context-window-strategy/prompts/summarize.md`).
   - Versionada em `summary_v{N}.md` para auditoria.
   - Budget default: 1000 tokens (`AGENT_SYNC_CTX_BUDGET`).

3. **Summarizer — 3 caminhos configuráveis**:

   | Caminho | Quando | Custo | Privacidade |
   | --- | --- | --- | --- |
   | **A. Próprio modelo** (default) | sempre que o agente tiver LLM na sessão | 1 chamada extra de LLM por compactação | dados vão pra API |
   | **B. Ollama local** (opt-in) | dados sensíveis | eletricidade | total |
   | **C. Heurística pura** (fallback) | quando A e B indisponíveis | zero | total |

   **Por que A é o default**: a compactação acontece exatamente quando o contexto está cheio e vai ser resetado. O próprio modelo tem contexto completo disponível, vai descartar o histórico logo em seguida, então o custo de tokens da sumarização **não é desperdiçado** (paper §4.4 confirma: overhead de +3.4% tokens vs C3, sem ganho de qualidade perdido).

4. **Idempotência e segurança**: hook best-effort, nunca bloqueia tool call (`set +e`, `catch {}`). Falha silenciosa em qualquer camada.

5. **Idioma da skill**: **inglês**. Decisão justificada em `memory-mcp` (`skill-english-default`): LLMs otimizam prompts técnicos em inglês. Trade-off aceito: quebra de consistência com regras globais PT-BR (não muda).

## Consequências

### Positivas

- **Ganho de conclusão**: paper reporta 91.6% (C4) vs 79% (C3) vs 71% (C2 / full context) em itemização automatizada no Dynamics 365.
- **Redução de tokens**: 63.9% menos que full context (paper §4.3).
- **Coerência mantida**: decisões e hipóteses sobrevivem viradas de janela via sumário.
- **Auditoria**: `summary_v{N}.md` preserva histórico de compactações para debug.
- **Reancoragem rápida**: sumário vira âncora leve entre CLIs e compactions.

### Negativas / trade-offs

- **Custo de API por compactação**: 1 chamada extra de LLM. Em sessões muito longas (milhares de compactações) pode ser significativo. Mitigação: fallback heurístico (C) ou Ollama (B).
- **Heurística limitada**: regex local captura apenas paths (artefatos estruturais), não semântica. **Validação empírica confirmou**: 6 seções vazias em tarefas com decisões explícitas. Heurística fica como fallback de segurança, não produção.
- **Best-effort no OpenCode**: hook `tool.execute.after` tem limitação upstream documentada ([anomalyco/opencode#13574](https://github.com/anomalyco/opencode/issues/13574)) — output do hook nem sempre chega ao modelo.
- **Inconsistência de idioma**: skill EN, regras globais PT-BR.

## Decisões revisadas

- **Summarizer não é mais fixo em `opencode run --pure`**: cada CLI (claude/codex/cursor/antigravity/opencode) resume via seu próprio modo não-interativo (`claude -p`, `codex exec`, `cursor-agent -p`, `agy -p`, `opencode run --pure`), confirmado ao vivo via `--help` de cada CLI instalada. Ver `summarizerCommand` em `summarize.go`.
- **Hooks de claude/codex/cursor/antigravity migraram de bash+Python para Go puro**: a primeira versão da captura de `tool_input`/`tool_response` usava um script `.py` por CLI (parsing JSON) invocado por um wrapper `.sh`. Motivo da reversão: misturar Go (tool principal) + bash (wrapper) + Python (parsing) no mesmo hook tornava difícil depurar e manter — três linguagens pra uma lógica que cabe em uma. Agora é um único subcomando `ctx-window hook <cli>` (Go), lendo o payload via stdin e chamando `on-tool-call-llm` internamente; `cmd/agent-sync/hooks.go` instala esse comando direto no settings.json, sem arquivo de script intermediário. **OpenCode é a única exceção que permanece em outra linguagem** (`hooks/ctx-compact.opencode.ts`): o hook dele *é* o runtime de plugin TS do OpenCode, não um comando de shell — não há equivalente Go pra substituir isso sem reescrever o próprio OpenCode.

## Evidência empírica (mini-projeto `/tmp/ctx-test/`)

6 runs com a mesma tarefa (refatorar `Calculator` introduzindo `Stats` struct, forçando decisão documentada):

| Cenário | Summarizer | Cap input | decisions | artifacts | resolved_errors | next_steps | Testes |
| --- | --- | --- | --- | --- | --- | --- | --- |
| A | nenhum (sem hook) | — | — | — | — | — | 5/5 ✓ |
| B | heurística | 200 | `[]` | paths | `[]` | `[]` | 5/5 ✓ |
| C | heurística | 50 (forçado) | `[]` | paths | `[]` | `[]` | 5/5 ✓ |
| D | heurística + cap 16000 | 16000 | `[]` | paths | `[]` | `[]` | 5/5 ✓ |
| E | heurística + cap 16000 | 16000 | `[]` | paths | `[]` | `[]` | 5/5 ✓ |
| **F** | **LLM (próprio modelo)** | 16000 | **5 decisões + rationale** | **paths + descrição semântica** | **cause + fix** | **2 passos + opcional** | **5/5 ✓** |

**Parecer final**:
- Heurística (Cenários B-E) → funciona para paths, falha para semântica. **Inadequada como caminho principal.**
- LLM summarizer (Cenário F) → **captura todas as 6 seções semanticamente**, incluindo restrições das regras globais (citou `AGENTS.md`). **Caminho real do paper validado.**
- Plugin TS funciona (OpenCode 6 hooks, 5 turns registrados por run).
- Auto-compact dispara corretamente (5-7 versões por run com threshold=80).
- Latência aceitável (1 chamada LLM extra por compactação, ~5s).

## Implementação

| Componente | Localização | Função |
| --- | --- | --- |
| Skill (protocolo) | `skills/context-window-strategy/SKILL.md` | Defaults, comandos, quando disparar |
| Prompt de sumarização | `skills/context-window-strategy/prompts/summarize.md` | Contrato de prompt LLM |
| Tool CLI | `tools/cmd/ctx-window/` | `show`, `compact` (heurística), `summarize`, `on-tool-call`, `on-tool-call-llm`, `hook <cli>`, `set-k`, `doctor`, `benchmark` |
| Heurística (fallback) | `tools/cmd/ctx-window/heuristic.go` | Regex bilíngue para paths |
| LLM summarizer | `tools/cmd/ctx-window/summarize.go` | Dispatch por `--cli` (`summarizerCommand`): `claude -p`, `codex exec`, `opencode run --pure`, `cursor-agent -p`, `agy -p` — cada CLI resume via seu próprio modo não-interativo, não só via opencode |
| Hook Claude/Codex/Cursor/Antigravity | `tools/cmd/ctx-window/hook.go` (`ctx-window hook <cli>`) | Lê o payload de PostToolUse via stdin, extrai `tool_input`/resultado real por schema de cada CLI (schemas confirmados lendo `docs-cache.py`/`docs-cache.cursor.py`/`docs-cache.antigravity.py`) e chama `on-tool-call-llm` internamente. Tudo em Go — decisão consciente após avaliar bash+Python por CLI (ver "Decisões revisadas" abaixo): mais difícil de depurar/manter com 3 linguagens no mesmo hook. |
| Plugin OpenCode (TS) | `hooks/ctx-compact.opencode.ts` | `tool.execute.after` (best-effort) — único que permanece fora do Go: o hook do OpenCode **é** o runtime de plugin TS, sem equivalente para trocar |
| Sync nas 5 CLIs | `cmd/agent-sync/hooks.go` (`syncCtxCompactHook`, `syncOpenCodeCtxCompactPlugin`) | Instalação automática via `-apply`; para claude/codex/cursor/antigravity instala o comando `ctx-window hook <cli>` diretamente no settings.json (sem script intermediário em disco) |

### Atualização de execução — 2026-09-18

1. **Eliminação do desperdício de tokens**: `on-tool-call-llm` agora apenas registra o turno em disco (`working memory`), sem chamadas aninhadas de LLM. O resumo estruturado é gerado sob demanda pelo comando `ctx-window summarize` (ou com detecção automática do projeto atual).
2. **Abordagem B — Handoff local por projeto (`.agent-sync/summary.md`)**:
   - `ctx-window summarize` salva o resumo do projeto em `<projectRoot>/.agent-sync/summary.md` (com `.gitignore` contendo `*` gerado automaticamente).
   - `ctx-window handoff` prioriza essa leitura local antes de inspecionar caches de sessões globais, garantindo isolamento total entre repositórios e branches.
3. **Mapeamento de Handoff e Nudges nas 5 CLIs** (status atualizado em 2026-09-19):
   - **Claude Code**: Handoff em `SessionStart` via `hookSpecificOutput.additionalContext`. Nudge em `PostToolUse` via transcript (`tools/cmd/ctx-window/claude_usage.go`).
   - **Codex**: Handoff em `SessionStart` via `hookSpecificOutput.additionalContext`. Nudge em `PostToolUse` via leitura dos rollouts JSONL locais (`~/.codex/sessions/` — `tools/cmd/ctx-window/codex_usage.go` + `hook.go:writeCodexNudge`). ✅ **fechado**
   - **Cursor**: Handoff em `sessionStart` via `additional_context`. Hook `preCompact` observacional orienta resumo manual. **Nudge de tokens**: tracking-only — `preCompact` é observacional e não pode modificar a compactação (`https://cursor.com/docs/hooks`); feature request oficial [#147216](https://forum.cursor.com/t/cursor-hooks-token-usage-support/147216) ativo desde 2025-12-24 (última resposta deanrie 2026-09-10: *"nothing has changed since June ... assume there won't be a built-in solution soon"*). Auto-monitor: `ctx-window track-docs`. Critério de reabertura em [PLAYBOOK-K-reopen.md](PLAYBOOK-K-reopen.md). 🔭
   - **Antigravity CLI**: Como o evento `SessionStart` é ignorado pelo runtime da CLI, migrou-se para o hook oficial `PreInvocation`. No turno 1 (`invocationNum == 1`), injeta o resumo e working memory via `ephemeralMessage` (sem poluir o transcript); nos turnos seguintes, retorna `{}` sem custo. **Nudge de tokens**: tracking-only — uso exposto apenas na API de status line, não em hook; aguardar `agy` documentar campo de tokens em payload. SDK Python tem issue precursora [#59](https://github.com/google-antigravity/antigravity-sdk-python/issues/59) ("built-in budget enforcement hooks") com `usage_metadata` já exposto em `ChatResponse`. Auto-monitor: `ctx-window track-docs`. 🔭
   - **OpenCode**: Plugin TS em `~/.config/opencode/plugins/ctx-compact.ts` registra turnos em `tool.execute.after` e injeta o snapshot local no array `output.context` em `experimental.session.compacting`. Nudge lê tokens da tabela `session` do SQLite local (`~/.local/share/opencode/opencode.db` — `tools/cmd/ctx-window/opencode_usage.go` + `main.go:checkOpenCodeNudge`). ✅ **fechado**
4. **Defensividade**: `runHandoff` e `runHook` retornam `{}` e exit code 0 em qualquer falha de leitura ou payload vazio, impedindo que falhas auxiliares quebrem a inicialização das CLIs.

## Limites conhecidos

1. **Mini-projeto não reproduz o paper completo** (50+ tool calls). Ganho percentual exato (91.6% vs 71%) **não foi medido** neste ambiente — exige Claude Opus / GPT-5 com sessão longa (custo proibitivo para validação local).
2. **K e teto do sumário aceitos do paper sem calibração local**. Decisão consciente, justificada na seção "Próximos passos".
3. **Hook não captura "thinking" do LLM** — apenas `args` + `output` da tool call.
4. **OpenCode plugin é best-effort** (upstream #13574).
5. **Modelo MiniMax M3 não degrada visivelmente em mini-tarefas** — efeito do sliding window não foi mensurável em cenário pequeno.

## Próximos passos (quando aplicável)

- **Fase 0 — calibração empírica**: executada em forma reduzida (mini-projeto `/tmp/ctx-test/`, 6 runs A–F). Variamos **summarizer** (heurística vs LLM) e **cap de input** (50/200/16000 chars). **Não variamos K nem teto do sumário** — aceitamos defaults do paper (K=5, summary_window=3, budget=1000 tokens) por dois motivos:
  1. **Custo proibitivo**: calibrar K e teto exigiria LLM que degrade em contexto longo + sessão real > 50 tool calls, fora do escopo deste ambiente.
  2. **Paper já calibrou**: arXiv 2606.10209v1 §4.4 documenta a calibração completa de W (summary window) com ganho de +12.6 pp sobre C3. Assumir os defaults do paper é defensável enquanto não houver evidência local em contrário.
- **Integração `memory-mcp`**: sumário vira memória `project` (`ctx-summary-<session_id>`) para reancorar entre CLIs. **Útil quando o usuário quiser reancorar uma sessão entre CLIs diferentes** — dependência opcional, não bloqueia o fluxo principal.

## Referências

- **Paper principal**: Lodha, A., Pahlavikhah Varnosfaderani, M., Chakraborty, A., Mithal, A. (2026). *Less Context, Better Agents: Efficient Context Engineering for Long-Horizon Tool-Using LLM Agents*. arXiv [2606.10209v1](https://arxiv.org/html/2606.10209v1). Microsoft.
  - C2 (full context): 71.0% conclusão, 1.480.996 tokens.
  - C3 (last 5 tool calls): 79.0%, 535.274 tokens (−63.9%).
  - C4 (last 5 + summary): 91.6%, ≈ 535k tokens (+3.4% vs C3).
  - Summary window W=3 (3 últimos antes do pruning boundary).
  - Single tool response: 500–3.000 tokens.
  - Sample summary: ~120 words (~150–200 tokens).
- **Upstream limitation**: [anomalyco/opencode#13574](https://github.com/anomalyco/opencode/issues/13574) — hook `tool.execute.after` best-effort.
- **Skills complementares**: `context-guard` (drift, compaction, STATE.md), `context-manager` (conceitual), `context-management-context-save` (genérica do catálogo).
- **Memória de decisão**: `memory-mcp` (`ctx-window-strategy-decision`, `skill-english-default`).
